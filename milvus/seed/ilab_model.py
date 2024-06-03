#!/bin/python3

## This is a langchain compatabible implementation for the Ilab models. It will remain in this repo until we publish APIKey 
## functionality and route backendservice endpoints through a proxy that can be exposed, similary to openAI. At which point
## we can move this pr as a contribution to langchain and easily scale our usage!

### Fixes in progress: 
    ### - override self params with calls invoke or generate for temperature, etc.
    ### - test that invoke works, generate starts
    ### - Feat: streaming implementation
    ### - Callbacks with streaming
    ### - Authentication enablement via user and password rather than just API keys
    ### - Authentication checking for API keys (whole backend API setup)
    ### - Utilize tags and metadata with langserve
    ### - Allow logprobs as an option

import os
import httpx
import requests
import json
from langchain_core.language_models.llms import BaseLLM
from dotenv import load_dotenv
from langchain_core.outputs import Generation, LLMResult
from langchain_core.pydantic_v1 import Field, SecretStr, root_validator
from langchain_core.utils import (
    convert_to_secret_str,
    get_from_dict_or_env,
    get_pydantic_field_names,
)
from langchain_core.utils.utils import build_extra_kwargs

load_dotenv()
from typing import (
    Any,
    Dict,
    List,
    Set,
    Optional,
    Mapping
)

class IlabLLM(BaseLLM):
    """
    Instructlab large language model.

    As this model is currently private, you must have pre-arranged access.
    """

    # REQUIRED PARAMS

    model_endpoint: str = ""
    """The model Endpoint to Use"""

    model_name: str = Field(alias="model")
    """Type of deployed model to use."""

    # OPTIONAL BUT DEFAULTS

    system_prompt: Optional[str] = "You are an AI language model developed by IBM Research. You are a cautious assistant. You carefully follow instructions. You are helpful and harmless and you follow ethical guidelines and promote positive behavior."
    """Default system prompt to use."""

    model_kwargs: Dict[str, Any] = Field(default_factory=dict)
    """Holds any model parameters valid for `create` call not explicitly specified."""

    max_tokens: int = 4096
    """The maximum number of tokens to generate in the completion.
    -1 returns as many tokens as possible given the prompt and
    the models maximal context size."""

    # TOTALLY OPTIONAL

    apikey: Optional[SecretStr] = None
    """Apikey to the Ilab model APIs (merlinte or granite)"""

    top_p: Optional[float] = 1
    """Total probability mass of tokens to consider at each step."""

    frequency_penalty: Optional[float] = 0
    """Penalizes repeated tokens according to frequency."""

    repetition_penalty: Optional[float] = 0
    """Penalizes repeated tokens."""

    temperature: Optional[float] = 0.7
    """What sampling temperature to use."""

    # verbose: Optional[str] = None
    # """If the model should return verbose output or standard"""

    streaming: bool = False
    """ Whether to stream the results or not. """

    # FUTURE EXTENSIONS

    tags: Optional[List[str]] = None
    """Tags to add to the run trace."""

    metadata: Optional[Dict[str, Any]] = None
    """Metadata to add to the run trace."""

    # This gets implemented with stream
    # callbacks: Optional[SecretStr] = None
    # """callbacks"""

    # END PARMS

    class Config:
        """Configuration for this pydantic object."""
        allow_population_by_field_name = True

    @property
    def lc_secrets(self) -> Dict[str, str]:
        """A map of constructor argument names to secret ids.

        For example:
            {
                "apikey": "ILAB_API_KEY",
            }
        """
        return {
            "apikey": "ILAB_API_KEY",
        }
    
    @classmethod
    def is_lc_serializable(cls) -> bool:
        """Return whether this model can be serialized by Langchain."""
        return False
    
    @root_validator(pre=True)
    def build_extra(cls, values: Dict[str, Any]) -> Dict[str, Any]:
        """Build extra kwargs from additional params that were passed in."""
        all_required_field_names = get_pydantic_field_names(cls)
        extra = values.get("model_kwargs", {})
        values["model_kwargs"] = build_extra_kwargs(
            extra, values, all_required_field_names
        )
        return values

    @root_validator()
    def validate_environment(cls, values: Dict) -> Dict:
        if values["streaming"] == True:
            raise ValueError("streaming has not yet been implemented.")
        if values["apikey"] or "ILAB_API_KEY" in os.environ:
            values["apikey"] = convert_to_secret_str(
                get_from_dict_or_env(values, "apikey", "ILAB_API_KEY")
            )
        values['model_name'] = get_from_dict_or_env(
            values,
            "model_name",
            "MODEL_NAME",
        )
        ## extension for more options for required auth params
        ## client_params = {
        ##     "api_key": (
        ##         values["apikey"].get_secret_value()
        ##         if values["apikey"]
        ##         else None
        ##     )
        ## }
        # CURRENTLY WE DONT CHECK KEYS
        ## if not client_params['values']['apikey']:
        ##     raise ValueError("Did not find token `apikey`.")
        return  values

    @property
    def _params(self) -> Mapping[str, Any]:
        """Get the identifying parameters."""
        params = {**{
            "model_name": self.model_name,
            "model_endpoint": self.model_endpoint,
        }, **self._default_params}
        if self.apikey:
            params['apikey'] = self.apikey
        if self.model_name:
            params['model_name'] = self.model_name
        return params
    
    @property
    def _default_params(self) -> Dict[str, Any]:
        """Get the default parameters for calling Merlinite API."""
        normal_params: Dict[str, Any] = {
            "temperature": self.temperature,
            "top_p": self.top_p,
            "frequency_penalty": self.frequency_penalty,
            "presence_penalty": self.repetition_penalty,
        }

        if self.max_tokens is not None:
            normal_params["max_tokens"] = self.max_tokens

        return {**normal_params, **self.model_kwargs}
    

    def _invocation_params(self) -> Dict[str, Any]:
        """Get the parameters used to invoke the model."""
        return self._params
    
    def make_request(self, params: Dict[str, Any], prompt: str, stop: Optional[List[str]]) -> Dict[str, Any]:
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.apikey}"
        }
        
        data = {
            "model": params['model_name'],
            "messages": [
                {
                    "role": "system",
                    "content": self.system_prompt
                },
                {
                    "role": "user",
                    "content": prompt
                }
            ],
            "temperature": params['temperature'],
            "max_tokens": params['max_tokens'],
            "top_p": params['top_p'],
            "stop": stop,
            "logprobs": False,
        }

        if 'repetition_penalty' in params:
            data["repetition_penalty"] = params['repetition_penalty']

        if 'streaming' in params:
            # Shadowing basemodel re-route for streaming
            data["stream"] = params["streaming"]

        response = requests.post(self.model_endpoint, headers=headers, data=json.dumps(data), verify=False)
        response_json = response.json()
    
    def _call(self, prompt: str, stop:Optional[List[str]] = None, **kwargs: Any) -> str:
        """Call the ilab inference endpoint. The result of invoke.
        Args:
            prompt: The prompt to pass into the model.
            stop: Optional list of stop words to use when generating.
            run_manager: Optional callback manager.
        Returns:
            The string generated by the model.
        Example:
            .. code-block:: python

                response = merlinite.invoke("What is a molecule")
        """

        invocation_params = self._invocation_params
        params = {**invocation_params, **kwargs}

        if stop == None:
            stop = ["<|endoftext|>"]
        response_json = self.make_request(
            params=params, prompt=prompt, stop=stop, **kwargs
        )
        return response_json['choices'][0]['messages']['content']

    def _generate(
        self,
        prompts: List[str],
        stop: Optional[List[str]] = None,
        **kwargs: Any,
    ) -> LLMResult:
        """Call out to Ilab's endpoint with prompt.

        Args:
            prompt: The prompt to pass into the model.
            stop: Optional list of stop words to use when generating.
            
        Returns:
            The full LLM output.

        Example:
            .. code-block:: python

                response = ilab.generate(["Tell me a joke."])
        """
        
        invocation_params = self._invocation_params()
        params = {**invocation_params, **kwargs}
        token_usage: Dict[str, int] = {}
        system_fingerprint: Optional[str] = None

        response_json = self.make_request(
            params=params, prompt=prompts[0], stop=stop, **kwargs
        )

        if not ('choices' in response_json and len(response_json['choices']) > 0):
            raise ValueError("No valid response from the model")

        if response_json.get("error"):
            raise ValueError(response_json.get("error"))

        if not system_fingerprint:
            system_fingerprint = response_json.get("system_fingerprint")
        return self._create_llm_result(
            response_json=response_json,
        )

    def _llm_type(self) -> str:
        """Get the type of language model used by this chat model. Used for logging purposes only."""
        return "instructlab"

    @property
    def max_context_size(self) -> int:
        """Get max context size for this model."""
        return self.modelname_to_contextsize(self.model_name)

    def _create_llm_result(self, response: List[dict]) -> LLMResult:
        """Create the LLMResult from the choices and prompt."""
        generations = []
        for res in response:
            results = res.get("results")
            if results:
                finish_reason = results[0].get("choices")[0].get('finished_reason')
                gen = Generation(
                    text=results[0].get("choices")[0].get('message').get('content'),
                    generation_info={"finish_reason": finish_reason},
                )
                generations.append([gen])
        final_token_usage = self._extract_token_usage(response)
        llm_output = {
            "token_usage": final_token_usage,
            "model_name": self.model_name
        }
        return LLMResult(generations=generations, llm_output=llm_output)

    @staticmethod
    def _extract_token_usage(
        response: Optional[List[Dict[str, Any]]] = None,
    ) -> Dict[str, Any]:
        if response is None:
            return {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}

        prompt_tokens = 0
        completion_tokens = 0
        total_tokens = 0

        def get_count_value(key: str, result: Dict[str, Any]) -> int:
            return result.get(key, 0) or 0

        for res in response:
            results = res.get("results")
            if results:
                prompt_tokens += get_count_value("prompt_tokens", results[0])
                completion_tokens += get_count_value(
                    "completion_tokens", results[0]
                )
                total_tokens += get_count_value("total_tokens", results[0])

        return {
            "prompt_tokens": prompt_tokens,
            "completion_tokens": completion_tokens,
            "total_tokens": total_tokens
        }

    @staticmethod
    def modelname_to_contextsize(modelname: str) -> int:
        """Calculate the maximum number of tokens possible to generate for a model."""
        model_token_mapping = {
            "ibm/merlinite-7b": 4096,
            "instructlab/granite-7b-lab": 4096
        }

        context_size = model_token_mapping.get(modelname, None)

        if context_size is None:
            raise ValueError(
                f"Unknown model: {modelname}. Please provide a valid Ilab model name."
                "Known models are: " + ", ".join(model_token_mapping.keys())
            )

        return context_size
