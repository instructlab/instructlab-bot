RAG application with ILAB

1. setup a vector DB (Milvus)

Development story:
    0. Starting Goal:
        - Naive RAG no KG aided
        - Addition: 
    1. identify what the model lacks knowledge in 
    2. Can I use the interal trained model or do I have to use the HF model
        - 

- UI integration

-----------------------------------------------

variable definition
class Config

_identify_params, 
_llm_type, _extract_token_usage, 

Inherint in defining this spec which could eventually live as a contribution to langchain are some assumptions / questions I made:
    - Is the model serializable: Assumed no
    - Max tokens for merlinite and granite: Both assumed 4096
    - Does this model have attention / memmory?
    - Does these models have a verbosity option for output?
    - Recomended default values:
        - 