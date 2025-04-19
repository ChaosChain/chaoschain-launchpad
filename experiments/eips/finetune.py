# Fine-Tune a Small Model on EIP Dataset using LoRA (Mac Friendly)

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

class ModelConfig:
    """Configuration for model training setup."""
    
    def __init__(self, model_name: str = "facebook/opt-125m"):
        """
        Initialize model configuration based on available hardware.
        
        Args:
            model_name: Name of the pre-trained model to use
        """
        self.model_name = model_name
        self.device = self._detect_device()
        self.dtype = torch.float16 if self.device == "cuda" else torch.float32
        self.use_fp16 = self.device == "cuda"
        
    def _detect_device(self) -> str:
        """Detect the best available device for training."""
        if torch.cuda.is_available():
            return "cuda"
        if torch.backends.mps.is_available():
            return "mps"
        return "cpu"
    
    def get_peft_config(self) -> LoraConfig:
        """Get LoRA configuration for the model."""
        return LoraConfig(
            task_type=TaskType.CAUSAL_LM,
            r=8,
            lora_alpha=16,
            lora_dropout=0.05,
            bias="none",
            target_modules=["q_proj", "v_proj"]
        )
    
    def get_training_args(self, output_dir: str) -> TrainingArguments:
        """Get training arguments optimized for the device."""
        return TrainingArguments(
            output_dir=output_dir,
            per_device_train_batch_size=1,
            gradient_accumulation_steps=4,
            num_train_epochs=3,
            logging_steps=10,
            save_strategy="epoch",
            learning_rate=2e-4,
            fp16=self.use_fp16,
            save_total_limit=1,
            ddp_find_unused_parameters=False,
            optim="adamw_torch",
            gradient_checkpointing=False
        )

class DatasetPreparation:
    """Handles dataset loading and preprocessing."""
    
    def __init__(self, tokenizer: AutoTokenizer, max_length: int = 512):
        """
        Initialize dataset preparation with tokenizer.
        
        Args:
            tokenizer: Tokenizer for encoding examples
            max_length: Maximum sequence length
        """
        self.tokenizer = tokenizer
        self.max_length = max_length
    
    def prepare_example(self, example: Dict[str, Any]) -> Dict[str, str]:
        """Format a single training example."""
        return {
            "input": f"{example['instruction']}\n\n{example['input']}",
            "output": example["output"]
        }
    
    def tokenize(self, example: Dict[str, str]) -> Dict[str, Any]:
        """Tokenize a prepared example."""
        full_text = example["input"] + example["output"]
        encoded = self.tokenizer(
            full_text,
            truncation=True,
            padding="max_length",
            max_length=self.max_length
        )
        encoded["labels"] = encoded["input_ids"].copy()
        return encoded
    
    def load_and_prepare(self, data_path: str):
        """Load and prepare the complete dataset."""
        dataset = load_dataset("json", data_files=data_path)
        processed = dataset.map(self.prepare_example)
        return processed.map(
            self.tokenize,
            remove_columns=processed["train"].column_names
        )

def setup_model(config: ModelConfig):
    """
    Set up the model with LoRA configuration.
    
    Args:
        config: Model configuration instance
        
    Returns:
        Tuple of (model, tokenizer)
    """
    model = AutoModelForCausalLM.from_pretrained(
        config.model_name,
        torch_dtype=config.dtype,
        device_map="auto",
        low_cpu_mem_usage=True
    )
    
    for param in model.parameters():
        param.requires_grad = False
    
    tokenizer = AutoTokenizer.from_pretrained(config.model_name)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token
    
    model = get_peft_model(model, config.get_peft_config())
    
    trainable_params = sum(p.numel() for p in model.parameters() if p.requires_grad)
    if trainable_params == 0:
        raise ValueError("No trainable parameters found! LoRA configuration might be incorrect.")
    
    return model, tokenizer

def train_model(
    data_path: str = "data/unified_training.jsonl",
    output_dir: str = "opt-eip-finetune",
    model_name: str = "facebook/opt-125m"
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
