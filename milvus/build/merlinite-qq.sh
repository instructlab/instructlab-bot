#!/bin/bash
## EXPECTED INPUT IS STRING ECAPSULATED
input="$1"
echo "input: $input"
request_body='{"model":"ibm/merlinite-7b","logprobs":false,"messages":[{"role": "system","content": "You are an AI language model developed by IBM Research. You are a cautious assistant. You carefully follow instructions. You are helpful and harmless and you follow ethical guidelines and promote positive behavior."},{"role":"user","content": "'$input'"}],"stream":false}'
echo $request_body
curl -X 'POST' 'https://merlinite-7b-vllm-openai.apps.fmaas-backend.fmaas.res.ibm.com/v1/chat/completions' -H 'accept: application/json' -H 'Content-Type: application/json' -k -d $request_body
