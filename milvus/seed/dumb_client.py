import requests
import json
import os
from dotenv import load_dotenv

load_dotenv()

# manage ENV
model_endpoint=os.getenv('MODEL_ENDPOINT')
if model_endpoint == "":
    model_endpoint = "http://localhost:8001"

model_name=os.getenv('MODEL_NAME')
if model_name == "":
    model_name = "ibm/merlinite-7b"

model_token=os.getenv('MODEL_TOKEN')

headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {model_token}"
}

data = {
    "model": model_name,
    "messages": [
        {"role": "system", "content": "your name is carl"},
        {"role": "user", "content": "what is your name?"}
    ],
    "temperature": 1,
    "max_tokens": 1792,
    "top_p": 1,
    "repetition_penalty": 1.05,
    "stop": ["<|endoftext|>"],
    "logprobs": False,
    "stream": False
}

response = requests.post(model_endpoint, headers=headers, data=json.dumps(data), verify=False)
print(response.json())