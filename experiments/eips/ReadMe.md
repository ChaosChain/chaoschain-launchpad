# To Ingest Data

```
python3 ingest.py --github-token <your-github-token>
```

# To Finetune

```
python3 finetune.py \
    --model-type llama \
    --model-path "meta-llama/Llama-2-7b-hf" \
    --data-path "data/my_training_data.jsonl" \
    --output-dir "llama2-eip-finetuned"
```

