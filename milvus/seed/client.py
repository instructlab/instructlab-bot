import requests
import json
import os
from ilab_model import IlabLLM
from dotenv import load_dotenv
from langchain_core.prompts import PromptTemplate
from langchain.chains import LLMChain

load_dotenv()

# manage ENV
model_endpoint=os.getenv('MODEL_ENDPOINT')
if model_endpoint == "":
    model_endpoint = "http://localhost:8001"

model_name=os.getenv('MODEL_NAME')
if model_name == "":
    model_name = "ibm/merlinite-7b"

model_token=os.getenv('ILAB_API_TOKEN')

# HTTPS client
# client_key_path = "/home/fedora/client-tls-key.pem2"
# client_crt_path = "/home/fedora/client-tls-crt.pem2"
# server_ca_crt   = "/home/fedora/server-ca-crt.pem2"

# ssl_context = ssl.create_default_context(cafile=server_ca_crt)
# ssl_context.load_cert_chain(certfile=client_crt_path, keyfile=client_key_path)

# client = httpx.Client(verify=ssl_context)

# data = {
#     "model": "instructlab/granite-7b-lab",
#     "messages": [
#         {"role": "system", "content": "your name is carl"},
#         {"role": "user", "content": "what is your name?"}
#     ],
#     "temperature": 1,
#     "max_tokens": 1792,
#     "top_p": 1,
#     "repetition_penalty": 1.05,
#     "stop": ["<|endoftext|>"],
#     "logprobs": False,
#     "stream": False
# }

# response = requests.post(url, headers=headers, data=json.dumps(data), verify=False)
# print(response.json())
print(f'model_name={model_name}')
llm = IlabLLM(
    model_endpoint=model_endpoint,
    model_name=model_name,
    apikey=model_token,
    temperature=1,
    max_tokens=500,
    top_p=1,
    repetition_penalty=1.05,
    stop=["<|endoftext|>"],
    streaming=False
)

prompt="I am training for a marathon in 12 weeks. Can you help me build an exercise plan to help prepare myself?"
prompts=[prompt]
# prompt_template = PromptTemplate.from_template(prompt)
llm.generate(prompts)
# llm.invoke("dog")
