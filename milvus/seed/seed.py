import os
from pymilvus import MilvusClient, DataType
from langchain_community.vectorstores import Milvus
from langchain_experimental.text_splitter import SemanticChunker
from langchain_community.document_loaders import PyPDFLoader, WebBaseLoader
from langchain_community.embeddings import HuggingFaceBgeEmbeddings, HuggingFaceInstructEmbeddings
from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain import hub
from langchain_core.runnables import RunnablePassthrough
from langchain_core.output_parsers import StrOutputParser
from tika import parser # pip install tika


def log_step(step_num, step_name) -> None:
    print("-----------------------------------------------")
    print(f"{step_num}. {step_name}")
    print("-----------------------------------------------")

def milvus_init() -> MilvusClient:
    client = MilvusClient()
    if not client.has_connection('dnd'):
        client.drop_connection('dnd')
    return client

def fill_dnd_collection(text_splitter: any, embeddings: any) -> None:
    # local
    raw = parser.from_file("data/DnD-5e-Handbook.pdf")
    print(len(raw['content']))
    docs = text_splitter.create_documents([raw['content']])
    vector_store = Milvus.from_documents(
        docs,
        embedding=embeddings,
        connection_args={"host": "localhost", "port": 19530},
        collection_name="dnd"
    )
    # remote
    # loader = PyPDFLoader('https://orkerhulen.dk/onewebmedia/DnD%205e%20Players%20Handbook%20%28BnW%20OCR%29.pdf')
    # data = loader.load()

def generate_embeddings() -> any:
    # model_name = "ibm/merlinite-7b"
    # model_kwargs={"device": "cuda"},
    # encode_kwargs = {"device": "cuda", "batch_size": 100, "normalize_embeddings": True}
    model_name = "all-MiniLM-L6-v2"
    model_kwargs = {"device": "cpu"}
    encode_kwargs = {"normalize_embeddings": True}
    embeddings = HuggingFaceBgeEmbeddings(
        model_name=model_name,
        # model_kwargs=model_kwargs,
        encode_kwargs=encode_kwargs,
        query_instruction = "search_query:",
        embed_instruction = "search_document:"
    )

def generate_text_splitter(chunk_size=512, chunk_overlap=50) -> any:
    # text_splitter = SemanticChunker(embeddings=embeddings) # fails 
    text_splitter = RecursiveCharacterTextSplitter(
        chunk_size=chunk_size,
        chunk_overlap=chunk_overlap,
        length_function=len,
        is_separator_regex=False
    )
    return text_splitter 

log_step(0, "Generate embeddings")
embeddings = generate_embeddings()
log_step(1, "Init text splitter")
text_splitter = generate_text_splitter()
log_step(2, "Read Raw data from PDF")
log_step(3, "Text splitting")
log_step(4, "Log result")
fill_dnd_collection(embeddings=embeddings, text_splitter=text_splitter)


# retreiver = vector_store.as_retreiver()
# prompt = hub.pull("rlm/rag-prompt")