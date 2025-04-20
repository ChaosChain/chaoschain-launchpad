import os
from dotenv import load_dotenv
import torch
from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    TrainingArguments,
    Trainer,
    DataCollatorForLanguageModeling
)
from peft import get_peft_model, LoraConfig, TaskType
from datasets import load_dataset
import gc
from typing import Dict, Any
import json
from datasets import Dataset
import argparse

class ModelConfig:
    """Configuration for model training setup."""
    
    DEEPSEEK = "deepseek"
    MISTRAL = "mistral"
    LLAMA = "llama"
    
    MODEL_CONFIGS = {
        DEEPSEEK: {
            "default_path": "deepseek-ai/deepseek-coder-6.7b-base",
            "target_modules": ["q_proj", "k_proj", "v_proj", "o_proj"],
            "lora_r": 16,
            "lora_alpha": 32,
            "modules_to_save": ["embed_tokens", "lm_head"]
        },
        MISTRAL: {
            "default_path": "mistralai/Mistral-7B-v0.1",
            "target_modules": ["q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"],
            "lora_r": 8,
            "lora_alpha": 16,
            "modules_to_save": ["embed_tokens", "lm_head"]
        },
        LLAMA: {
            "default_path": "meta-llama/Llama-2-7b-hf",
            "target_modules": ["q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"],
            "lora_r": 8,
            "lora_alpha": 16,
            "modules_to_save": ["embed_tokens", "lm_head"]
        }
    }
    
    def __init__(self, model_path: str = None, model_type: str = None):
        """
        Initialize model configuration based on available hardware.
        
        Args:
            model_path: Path to model or HF repo (optional)
            model_type: Type of model architecture (deepseek/mistral/llama)
        """
    
        if model_type is None:
            model_type = self._detect_model_type(model_path)
        
        self.model_type = model_type
        self.model_name = model_path or self.MODEL_CONFIGS[model_type]["default_path"]
        self.device = self._detect_device()
        self.dtype = torch.bfloat16 if self.device == "cuda" else torch.float32
        self.use_fp16 = False
        self.max_length = 4096 if model_type in [self.MISTRAL, self.LLAMA] else 2048
    
    def _detect_model_type(self, model_path: str) -> str:
        """Detect model type from path."""
        if not model_path:
            return self.DEEPSEEK
        
        path_lower = model_path.lower()
        if "mistral" in path_lower:
            return self.MISTRAL
        elif "llama" in path_lower:
            return self.LLAMA
        elif "deepseek" in path_lower:
            return self.DEEPSEEK
        else:
            print(f"Warning: Unknown model type for {model_path}, defaulting to DeepSeek config")
            return self.DEEPSEEK
    
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
        """Get LoRA configuration optimized for the specific model type."""
        model_config = self.MODEL_CONFIGS[self.model_type]
        
        return LoraConfig(
            task_type=TaskType.CAUSAL_LM,
            r=model_config["lora_r"],
            lora_alpha=model_config["lora_alpha"],
            lora_dropout=0.1,
            bias="none",
            target_modules=model_config["target_modules"],
            modules_to_save=model_config["modules_to_save"]
        )
    
    def get_training_args(self, output_dir: str) -> TrainingArguments:
        """Get training arguments optimized for the device and model."""
        
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
        
        if self.model_type in [self.MISTRAL, self.LLAMA]:
            args.update({
                "max_steps": 1500,
                "warmup_ratio": 0.05,
            })
        
        if self.device == "cuda":
            args.update({
                "per_device_train_batch_size": 1,
                "gradient_accumulation_steps": 32,
                "learning_rate": 5e-5 if self.model_type == self.DEEPSEEK else 3e-5,
                "bf16": True,
                "tf32": True,
                "gradient_checkpointing": True,
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
        
        def tokenize_function(examples):
            tokenized = self.tokenizer(
                examples['text'],
                truncation=True,
                max_length=2048,
                padding=True,
                return_tensors=None
            )
            
            tokenized['labels'] = tokenized['input_ids'].copy()
            
            if 'attention_mask' not in tokenized:
                tokenized['attention_mask'] = [1] * len(tokenized['input_ids'])
            
            return tokenized
        
        tokenized_dataset = dataset.map(
            tokenize_function,
            remove_columns=['text'],
            batched=True,
            desc="Tokenizing dataset"
        )
        
        train_test = tokenized_dataset.train_test_split(
            test_size=0.05, 
            shuffle=True, 
            seed=42
        )
        
        return train_test

def setup_model(config: ModelConfig, hf_token: str):
    """Set up the model with LoRA configuration."""
    
    print(f"\nLoading model {config.model_name}...")
    
    if not hf_token:
        raise ValueError("Please set HUGGING_FACE_TOKEN environment variable")
    
    try:
        # Load tokenizer first
        tokenizer = AutoTokenizer.from_pretrained(
            config.model_name,
            trust_remote_code=True,
            use_fast=False,
            padding_side="right",
            token=hf_token
        )
        if tokenizer.pad_token is None:
            tokenizer.pad_token = tokenizer.eos_token
        
        # Then load model with the tokenizer's vocab size
        model_args = {
            "torch_dtype": config.dtype,
            "low_cpu_mem_usage": True,
            "trust_remote_code": True,
            "use_cache": False,
            "token": hf_token
        }
        
        if config.device == "cuda":
            model_args["device_map"] = "auto"
        else:
            model_args["device_map"] = None
            
        model = AutoModelForCausalLM.from_pretrained(
            config.model_name,
            **model_args
        )
        
        if config.device != "cuda":
            model = model.to(config.device)
            
    except Exception as e:
        print(f"Error loading model: {e}")
        raise e
    
    # Freeze base model parameters
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
    output_dir: str = None,
    model_path: str = None,
    model_type: str = None,
    hf_token: str = None
):
    """
    Train the model on the prepared dataset.
    
    Args:
        data_path: Path to training data
        output_dir: Directory to save the model (will be auto-generated if None)
        model_path: Path to model or HF repo (optional)
        model_type: Type of model (deepseek/mistral/llama)
    """
    load_dotenv()
    gc.collect()
    torch.cuda.empty_cache() if torch.cuda.is_available() else None
    
    config = ModelConfig(model_path, model_type)
    
    if output_dir is None:
        model_name = config.model_name.split('/')[-1]
        output_dir = f"{model_name}-eip-finetune"
    
    print(f"Training {config.model_type} model: {config.model_name}")
    print(f"Using device: {config.device}")
    print(f"Using dtype: {config.dtype}, fp16: {config.use_fp16}")
    
    model, tokenizer = setup_model(config, hf_token)
    model.print_trainable_parameters()
    
    dataset_prep = DatasetPreparation(tokenizer)
    tokenized_dataset = dataset_prep.load_and_prepare(data_path)
    
    data_collator = DataCollatorForLanguageModeling(
        tokenizer=tokenizer,
        mlm=False
    )
    
    trainer = Trainer(
        model=model,
        args=config.get_training_args(output_dir),
        train_dataset=tokenized_dataset["train"],
        data_collator=data_collator,
        tokenizer=tokenizer
    )
    
    trainer.train()
    
    model.save_pretrained(output_dir)
    tokenizer.save_pretrained(output_dir)

def parse_args():
    """Parse command line arguments."""
    parser = argparse.ArgumentParser(description='Fine-tune language models on EIP dataset')
    
    parser.add_argument(
        '--model-type',
        type=str,
        choices=['deepseek', 'mistral', 'llama'],
        default='deepseek',
        help='Type of model to fine-tune (default: deepseek)'
    )
    
    parser.add_argument(
        '--model-path',
        type=str,
        help='Path to local model or Hugging Face model ID. If not provided, uses default path for model type.'
    )
    
    parser.add_argument(
        '--data-path',
        type=str,
        default='data/unified_training.jsonl',
        help='Path to training data (default: data/unified_training.jsonl)'
    )
    
    parser.add_argument(
        '--output-dir',
        type=str,
        help='Directory to save the fine-tuned model. If not provided, auto-generates based on model name.'
    )
    
    parser.add_argument(
        '--hf-token',
        type=str,
        help='Hugging Face token for authentication'
    )
    
    return parser.parse_args()

if __name__ == "__main__":
    args = parse_args()
    
    train_model(
        data_path=args.data_path,
        output_dir=args.output_dir,
        model_path=args.model_path,
        model_type=args.model_type,
        hf_token=args.hf_token
    )
