from pydantic import BaseModel
from typing import List, Optional, Any
from langchain.schema import Document

class AgentState(BaseModel):
    documents: Optional[List[Document]] = None
    clean_docs: Optional[List[Document]] = None
    vectorstore: Optional[Any] = None
    similar_docs: Optional[List[Document]] = None
    friction_themes: Optional[List[dict]] = None
    status: Optional[str] = None
    path: Optional[str] = None