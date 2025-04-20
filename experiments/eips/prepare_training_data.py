from typing import List, Dict, Any, Optional
import json
from tqdm import tqdm
import os

class TrainingTemplates:
    """Templates for different types of training examples."""
    
    eip_proposal = {
        "instruction": "Write a detailed EIP proposal based on the following information and context.",
        "input_format": """Title: {title}
EIP Number: {eip}
Authors: {authors}
Context: {context}

Write a complete EIP proposal following the standard format.""",
        "output_format": """# {title}

## Abstract
{abstract}

## Motivation
{motivation}

## Specification
{specification}

## Rationale
{rationale}

## Security Considerations
{security}"""
    }
    
    ethereum_concept = {
        "instruction": "Explain the following Ethereum concept in detail.",
        "input_format": "Concept: {title}\nContext: {context}",
        "output_format": "{explanation}"
    }
    
    discussion_response = {
        "instruction": "Write a response to the following {type}.",
        "input_format": """Type: {type}
Title: {title}
Context: {context}
Labels: {labels}
Status: {status}""",
        "output_format": "{response}"
    }

def format_training_example(source: str, data: Dict[str, Any]) -> Dict[str, str]:
    """Format data from different sources into a consistent training example format."""
    
    if source == "eip":
        # Combine sections into content
        content = "\n\n".join(f"## {section}\n{text}" for section, text in data["sections"].items())
        
        return {
            "instruction": TrainingTemplates.eip_proposal["instruction"],
            "input": TrainingTemplates.eip_proposal["input_format"].format(
                title=data["title"],
                eip=data["eip"],
                authors=", ".join(data["authors"]), # Join author list
                context=data.get("context", "")
            ),
            "output": content,
            "metadata": {
                "source": "eip",
                "type": data.get("content_type", "proposal"),
                "eip": data["eip"]
            }
        }
    
    if source == "book":
        try:
            return {
                "instruction": TrainingTemplates.ethereum_concept["instruction"],
                "input": TrainingTemplates.ethereum_concept["input_format"].format(
                    title=data.get("title", ""),
                    context=data.get("context", "")
                ),
                "output": data.get("content", ""),
                "metadata": {
                    "source": "ethereumbook",
                    "chapter": data.get("chapter", "unknown"),
                    "section": data.get("section", "unknown")
                }
            }
        except Exception as e:
            raise Exception(f"Failed to format book example: {str(e)}, data: {data}")
    
    if source == "discussion":
        return {
            "instruction": TrainingTemplates.discussion_response["instruction"].format(
                type=data["type"]
            ),
            "input": TrainingTemplates.discussion_response["input_format"].format(
                type=data["type"],
                title=data["title"],
                context=data.get("context", ""),
                labels=", ".join(data["labels"]),
                status=data["state"]
            ),
            "output": data["body"] if data["body"] else "",
            "metadata": {
                "source": "github",
                "type": data["type"],
                "number": data["number"],
                "state": data["state"]
            }
        }
    
    raise ValueError(f"Unknown source type: {source}")

def prepare_unified_training_format(data_dir: str, output_path: str) -> List[Dict[str, Any]]:
    """
    Prepare a unified training dataset from all sources with consistent formatting.
    
    Args:
        data_dir: Directory containing source data files
        output_path: Path to save the unified training dataset
        
    Returns:
        List of formatted training examples
    """
    training_examples = []
    source_files = {
        "eip": "eips.jsonl",
        "book": "ethereum_book.jsonl",
        "discussion": "github_discussions.jsonl"
    }
    
    for source, filename in source_files.items():
        filepath = os.path.join(data_dir, filename)
        if not os.path.exists(filepath):
            print(f"Warning: {filepath} not found, skipping...")
            continue
            
        with open(filepath, "r") as f:
            for line in tqdm(f, desc=f"Processing {source}"):
                data = json.loads(line)
                try:
                    example = format_training_example(source, data)
                    training_examples.append(example)
                except Exception as e:
                    print(f"Error processing {source} example: {e}")
                    continue
    
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w") as f:
        for example in tqdm(training_examples, desc="Writing unified dataset"):
            f.write(json.dumps(example) + "\n")
    
    return training_examples 