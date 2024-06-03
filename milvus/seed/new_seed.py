import os
from pymilvus import MilvusClient, DataType
from langchain_community.vectorstores import Milvus
from langchain_experimental.text_splitter import SemanticChunker
from langchain_community.document_loaders import PyPDFLoader, WebBaseLoader
from langchain_community.embeddings import HuggingFaceBgeEmbeddings, HuggingFaceInstructEmbeddings, HuggingFaceEmbeddings
from langchain.text_splitter import RecursiveCharacterTextSplitter, CharacterTextSplitter
from langchain import hub
from langchain_core.runnables import RunnablePassthrough
from langchain_core.output_parsers import StrOutputParser
from tika import parser # pip install tika
from langchain_openai import OpenAI
from ilab_models import IlabOpenAILLM


def log_step(step_num, step_name) -> None:
    print("-----------------------------------------------")
    print(f"{step_num}. {step_name}")
    print("-----------------------------------------------")

embeddings = HuggingFaceEmbeddings(model_name="all-MiniLM-L6-v2")

text_splitter = SemanticChunker(embeddings=embeddings) # fails 

loader = PyPDFLoader('./data/DnD-5e-Handbook.pdf')
data = loader.load()
split_data = text_splitter.split_documents(data)
print(len(split_data))
vector_store = Milvus.from_documents(
    documents=split_data,
    embedding=embeddings,
    connection_args={"host": "localhost", "port": 19530},
    collection_name="dnd"
)

llm = IlabOpenAILLM(
    
)

retreiver = vector_store.as_retreiver()
prompt = hub.pull("rlm/rag-prompt")