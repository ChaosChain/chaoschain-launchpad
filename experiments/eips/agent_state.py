from pydantic import BaseModel, Field
from typing import List, Dict, Any, Optional
from langchain.schema import Document
from langchain_chroma import Chroma

class AgentState(BaseModel):
    documents: Optional[List[Any]] = Field(default_factory=list)
    clean_docs: Optional[List[Any]] = Field(default_factory=list)
    vectorstore: Optional[Any] = None
    similar_docs: Optional[List[Any]] = Field(default_factory=list)
    friction_themes: Optional[List[Dict[str, Any]]] = Field(default_factory=list)
    status: Optional[str] = None
    path: Optional[str] = None
    proposal_draft: Optional[str] = None
    
    class Config:
        arbitrary_types_allowed = True