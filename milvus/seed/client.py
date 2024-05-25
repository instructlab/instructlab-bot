import httpx
import os
import ssl

# manage ENV
model_endpoint=os.getenv('MODEL_ENDPOINT')
if model_endpoint == "":
    model_endpoint = "http://localhost:8001"

model_name=os.getenv('MODEL_NAME')
if model_name == "":
    model_name = "ibm/merlinite-7b"

model_token=os.getenv('MODEL_TOKEN')

# HTTPS client
client_key_path = "/home/fedora/client-tls-key.pem2"
client_crt_path = "/home/fedora/client-tls-crt.pem2"
server_ca_crt   = "/home/fedora/server-ca-crt.pem2"

ssl_context = ssl.create_default_context(cafile=server_ca_crt)
ssl_context.load_cert_chain(certfile=client_crt_path, keyfile=client_key_path)

client = httpx.Client(verify=ssl_context)


def get_openai_response(prompt, **kwargs):
    url = model_endpoint
    headers = {
        "Authorization": f"Bearer {model_token}",
        "Content-Type": "application/json"
    }
    data = {
        "model": model_name,
        "max_tokens": 4096,
        "messages": [
            {
                "role": "system",
                "content": "You are an AI language model developed by IBM Research. You are a cautious assistant that carefully follows instructions. You are helpful and harmless and you follow ethical guidelines and promote positive behavior."
            },
            {
                "role":"user",
                "content": prompt
            }
        ],
        "logprobs":False,
        "stream":False
    }

    response = client.post(url, json=data, headers=headers)
    response.raise_for_status()
    return response.json()

question = """ Question: I am training for an upcoming marathon but I am completely out of shape! Can you help me to implement a plan to prepare me for running a marathon in 12 weeks?

Answer: Let's think step by step. """

# get_openai_response(question)
