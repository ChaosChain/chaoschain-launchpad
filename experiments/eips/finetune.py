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
from peft import get_peft_model, LoraConfig, TaskType, prepare_model_for_kbit_training
from datasets import load_dataset
import gc

# Load Hugging Face token if using private models
load_dotenv()

# Detect device and set appropriate configuration
device = "cuda" if torch.cuda.is_available() else "mps" if torch.backends.mps.is_available() else "cpu"
print(f"Using device: {device}")

# Set appropriate dtype and fp16 flag based on device
if device == "cuda":
    dtype = torch.float16
    use_fp16 = True
else:
    dtype = torch.float32
    use_fp16 = False
    
print(f"Using dtype: {dtype}, fp16: {use_fp16}")

# Force garbage collection
gc.collect()
torch.cuda.empty_cache() if torch.cuda.is_available() else None

# Use a smaller model for Mac
model_name = "facebook/opt-125m"  # Small model that works well with LoRA

print(f"Loading model: {model_name}")
model = AutoModelForCausalLM.from_pretrained(
    model_name,
    torch_dtype=dtype,
    device_map="auto",
    low_cpu_mem_usage=True
)

# Ensure model parameters are properly set up for training
for param in model.parameters():
    param.requires_grad = False  # Freeze all parameters first

tokenizer = AutoTokenizer.from_pretrained(model_name)

# Make sure pad_token is set
if tokenizer.pad_token is None:
    tokenizer.pad_token = tokenizer.eos_token

# Use appropriate target modules for OPT model
peft_config = LoraConfig(
    task_type=TaskType.CAUSAL_LM,
    r=8,
    lora_alpha=16,
    lora_dropout=0.05,
    bias="none",
    target_modules=["q_proj", "v_proj"]  # For OPT architecture
)

# Apply LoRA
model = get_peft_model(model, peft_config)

# Verify trainable parameters
trainable_param_count = sum(p.numel() for p in model.parameters() if p.requires_grad)
print(f"Trainable parameters: {trainable_param_count}")
if trainable_param_count == 0:
    raise ValueError("No trainable parameters found! LoRA configuration might be incorrect.")

model.print_trainable_parameters()

# Load dataset
dataset = load_dataset("json", data_files="core_eips.jsonl")

# Convert EIP data to input-output pairs
def process_eip_data(example):
    input_text = f"EIP-{example['eip']}: {example['title']}\nAuthors: {example['authors']}\n\nWrite a detailed EIP proposal based on this information."
    output_text = ""
    if 'sections' in example:
        for section_title, section_content in example['sections'].items():
            output_text += f"## {section_title}\n{section_content}\n\n"
    return {
        "input": input_text,
        "output": output_text
    }

processed_dataset = dataset.map(process_eip_data)

# Tokenization with shorter sequence length
def tokenize(example):
    # Format based on model's expected format
    prompt = example['input']
    response = example["output"]
    full = prompt + response

    encoded = tokenizer(
        full,
        truncation=True,
        padding="max_length",
        max_length=512  # Even shorter for small model
    )
    encoded["labels"] = encoded["input_ids"].copy()
    return encoded

tokenized_dataset = processed_dataset.map(tokenize, remove_columns=processed_dataset["train"].column_names)

# Training args with memory optimizations
training_args = TrainingArguments(
    output_dir="opt-eip-finetune",
    per_device_train_batch_size=1,
    gradient_accumulation_steps=4,
    num_train_epochs=3,
    logging_steps=10,
    save_strategy="epoch",
    learning_rate=2e-4,
    fp16=use_fp16,
    save_total_limit=1,  # Save less checkpoints
    ddp_find_unused_parameters=False,
    optim="adamw_torch",  # Use standard optimizer instead of 8-bit
    gradient_checkpointing=False,  # Disable gradient checkpointing as it's causing issues
)

trainer = Trainer(
    model=model,
    args=training_args,
    train_dataset=tokenized_dataset["train"],
)

trainer.train()

model.save_pretrained("opt-eip-proposer")
tokenizer.save_pretrained("opt-eip-proposer")
