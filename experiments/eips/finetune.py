import os
from dotenv import load_dotenv
import torch
from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    TrainingArguments,
    Trainer
)
from peft import get_peft_model, LoraConfig, TaskType
from datasets import load_dataset
import gc
from typing import Dict, Any
import json
from datasets import Dataset

class ModelConfig:
    """Configuration for model training setup."""
    
    def __init__(self, model_name: str = "deepseek-ai/deepseek-coder-6.7b-base"):
        """
        Initialize model configuration based on available hardware.
        
        Args:
            model_name: Name of the pre-trained model to use (default: deepseek-coder-6.7b-base)
        """
        self.model_name = model_name
        self.device = self._detect_device()
        self.dtype = torch.bfloat16 if self.device == "cuda" else torch.float32
        self.use_fp16 = False
        self.max_length = 2048
        
    def _detect_device(self) -> str:
        """Detect the best available device for training."""
        if torch.cuda.is_available():
            print("GPU detected - using CUDA for training")
            return "cuda"
        if torch.backends.mps.is_available():
            print("Apple Silicon detected - using MPS for training")
            return "mps"
        print("No GPU detected - falling back to CPU training (this will be slow)")
        return "cpu"
    
    def get_peft_config(self) -> LoraConfig:
        """Get LoRA configuration optimized for DeepSeek models."""
        return LoraConfig(
            task_type=TaskType.CAUSAL_LM,
            r=16,
            lora_alpha=32,
            lora_dropout=0.1,
            bias="none",
            target_modules=["q_proj", "k_proj", "v_proj", "o_proj"],
            modules_to_save=["embed_tokens", "lm_head"]
        )
    
    def get_training_args(self, output_dir: str) -> TrainingArguments:
        """Get training arguments optimized for the device."""
        
        args = {
            "output_dir": output_dir,
            "num_train_epochs": 2,
            "logging_steps": 5,
            "save_strategy": "steps",
            "save_steps": 100,
            "save_total_limit": 2,
            "ddp_find_unused_parameters": False,
            "optim": "paged_adamw_32bit",
            "lr_scheduler_type": "cosine",
            "warmup_ratio": 0.03,
            "max_grad_norm": 0.3,
            "group_by_length": True,
        }
        
        if self.device == "cuda":
            args.update({
                "per_device_train_batch_size": 2,
                "gradient_accumulation_steps": 16,
                "learning_rate": 5e-5,
                "bf16": True,
                "tf32": True,
                "gradient_checkpointing": True,
                "max_steps": 1000
            })
        elif self.device == "mps":
            args.update({
                "per_device_train_batch_size": 1,
                "gradient_accumulation_steps": 32,
                "learning_rate": 2e-5,
                "fp16": False,
                "gradient_checkpointing": True
            })
        else:
            args.update({
                "per_device_train_batch_size": 1,
                "gradient_accumulation_steps": 64,
                "learning_rate": 1e-5,
                "fp16": False,
                "gradient_checkpointing": False
            })
        
        return TrainingArguments(**args)

class DatasetPreparation:
    """Prepare dataset for training."""
    
    def __init__(self, tokenizer):
        self.tokenizer = tokenizer
        
    def load_and_prepare(self, data_path: str):
        """Load and prepare dataset for training."""
        dataset = []
        with open(data_path, 'r') as f:
            for line in f:
                dataset.append(json.loads(line))
                
        dataset = Dataset.from_list(dataset)
        
        dataset = dataset.map(
            lambda x: {
                'text': (
                    f"### Instruction:\n{x['instruction']}\n\n"
                    f"### Input:\n{x['input']}\n\n"
                    f"### Response:\n{x['output']}\n\n"
                    f"### End\n"
                )
            }
        )
        
        dataset = dataset.map(
            lambda x: self.tokenizer(
                x['text'],
                truncation=True,
                max_length=2048,
                padding='max_length',
                return_tensors='pt'
            ),
            remove_columns=['text']
        )
        
        return dataset.train_test_split(test_size=0.05, shuffle=True, seed=42)

def setup_model(config: ModelConfig):
    """Set up the model with LoRA configuration."""
    
    print(f"\nLoading model {config.model_name}...")
    
    model_args = {
        "torch_dtype": config.dtype,
        "low_cpu_mem_usage": True,
        "trust_remote_code": True
    }
    
    if config.device == "cuda":
        model_args["device_map"] = "auto"
    else:
        model_args["device_map"] = None
    
    try:
        tokenizer = AutoTokenizer.from_pretrained(
            config.model_name,
            trust_remote_code=True,
            use_fast=False
        )
        if tokenizer.pad_token is None:
            tokenizer.pad_token = tokenizer.eos_token
            
        model = AutoModelForCausalLM.from_pretrained(
            config.model_name,
            **model_args
        )
        
        if config.device != "cuda":
            model = model.to(config.device)
            
    except Exception as e:
        print(f"Error loading model: {e}")
        print("Attempting to load with safetensors disabled...")
        model_args["use_safetensors"] = False
        model = AutoModelForCausalLM.from_pretrained(
            config.model_name,
            **model_args
        )
    
    for param in model.parameters():
        param.requires_grad = False
    
    print("Applying LoRA adapters...")
    model = get_peft_model(model, config.get_peft_config())
    
    trainable_params = sum(p.numel() for p in model.parameters() if p.requires_grad)
    if trainable_params == 0:
        raise ValueError("No trainable parameters found! LoRA configuration might be incorrect.")
    
    print(f"Model loaded successfully on {config.device}")
    return model, tokenizer

def train_model(
    data_path: str = "data/unified_training.jsonl",
    output_dir: str = "deepseek-eip-finetune",
    model_name: str = "deepseek-ai/deepseek-coder-6.7b-base"
):
    """
    Train the model on the prepared dataset.
    
    Args:
        data_path: Path to the training data
        output_dir: Directory to save the model
        model_name: Name of the pre-trained model to use
    """
    load_dotenv()
    gc.collect()
    torch.cuda.empty_cache() if torch.cuda.is_available() else None
    
    config = ModelConfig(model_name)
    print(f"Using device: {config.device}")
    print(f"Using dtype: {config.dtype}, fp16: {config.use_fp16}")
    
    model, tokenizer = setup_model(config)
    model.print_trainable_parameters()
    
    dataset_prep = DatasetPreparation(tokenizer)
    tokenized_dataset = dataset_prep.load_and_prepare(data_path)
    
    trainer = Trainer(
        model=model,
        args=config.get_training_args(output_dir),
        train_dataset=tokenized_dataset["train"]
    )
    
    trainer.train()
    
    model.save_pretrained(output_dir)
    tokenizer.save_pretrained(output_dir)

if __name__ == "__main__":
    train_model()
