import os
from pymilvus import MilvusClient, DataType
from langchain_experimental.text_splitter import SemanticChunker
from langchain_community.document_loaders import PyPDFLoader, WebBaseLoader
from langchain_community.embeddings import HuggingFaceBgeEmbeddings, HuggingFaceInstructEmbeddings
from tika import parser # pip install tika

def log_step(step_num, step_name) -> None:
    print("-----------------------------------------------")
    print(f"{step_num}. {step_name}")
    print("-----------------------------------------------")

# model_name = "ibm/merlinite-7b"
# model_kwargs = {"device": "cpu"}
# encode_kwargs = {"normalize_embeddings": True}

model_name = "ibm/merlinite-7b"
model_kwargs={"device": "cuda"},
encode_kwargs = {"device": "cuda", "batch_size": 100, "normalize_embeddings": True}

log_step(0, "Generate embeddings")
embeddings = HuggingFaceBgeEmbeddings(
    model_name=model_name,
    model_kwargs=model_kwargs,
    encode_kwargs=encode_kwargs,
    query_instruction = "search_query:",
    embed_instruction = "search_document:"
)

log_step(1, "Init text splitter")
text_splitter = SemanticChunker(embeddings=embeddings)
log_step(2, "Read Raw data from PDF")
raw = parser.from_file("data/DnD-5e-Handbook.pdf")
log_step(3, "Text splitting")
print(len(raw['content']))
docs = text_splitter.create_documents([raw['content']])
log_step(4, "Log result")
print(len(docs))