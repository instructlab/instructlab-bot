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

model_name = "ibm/merlinite-7b"
model_kwargs = {"device": "cpu"}
encode_kwargs = {"normalize_embeddings": True}

log_step(0, "Generate embeddings")
embeddings = HuggingFaceBgeEmbeddings(
    model_name=model_name,
    model_kwargs=model_kwargs,
    encode_kwargs=encode_kwargs,
    query_instruction = "search_query:",
    embed_instruction = "search_document:"
)


# data_url = "https://orkerhulen.dk/onewebmedia/DnD%205e%20Players%20Handbook%20%28BnW%20OCR%29.pdf"
# loader = WebBaseLoader(data_url)
# data = loader.load()
raw = parser.from_file("data/DnD-5e-Handbook.pdf")
print(raw['content'])
