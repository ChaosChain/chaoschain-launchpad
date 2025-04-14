from langgraph.graph import StateGraph
from langchain_huggingface import HuggingFaceEmbeddings
from langchain_chroma import Chroma
from langchain.schema import Document
import re, os, json
from agent_state import AgentState
from ingest_data import ingest_core_eips

# ---------- Step 1: Load EIP Inputs ----------

def load_eip_documents(path="core_eips.jsonl"):
    documents = []
    with open(path, "r") as f:
        for line in f:
            eip = json.loads(line)
            for section_title, section_text in eip["sections"].items():
                if section_text.strip():
                    documents.append(
                        Document(
                            page_content=section_text,
                            metadata={
                                "title": eip["title"],
                                "section": section_title,
                                "url": eip["url"],
                                "eip": eip["eip"]
                            }
                        )
                    )
    return documents


# ---------- Step 2: Preprocess ----------
def preprocess_node(state):
    state_dict = state.model_dump()
    state_dict.pop("clean_docs", None)

    return AgentState(clean_docs=state.documents, **state_dict)

# ---------- Step 3: Embed & Store ----------
embedding_model = HuggingFaceEmbeddings(model_name="sentence-transformers/all-mpnet-base-v2")
if not os.path.exists("chroma_index"):
    os.mkdir("chroma_index")

def embed_node(state):
    embedding_model = HuggingFaceEmbeddings(model_name="sentence-transformers/all-mpnet-base-v2")
    processed_state = preprocess_node(state) 
    vectorstore = Chroma.from_documents(processed_state.clean_docs, embedding_model, persist_directory="chroma_index")  
    
    return AgentState(vectorstore=vectorstore, documents=processed_state.clean_docs, **state.model_dump(exclude={"vectorstore", "documents"}))

# ---------- Step 4: Retrieve Similar ----------
def retrieve_node(state):
    retriever = state.vectorstore.as_retriever()
    query = "What design issues exist with proposer-builder separation or blob transaction latency?"
    results = retriever.get_relevant_documents(query)
    return AgentState(similar_docs=results, **state.model_dump(exclude={"similar_docs"}))


# ---------- Step 5: Classify Theme ----------
def classify_node(state):
    # Covers 90%+ of relevant EIP areas
    theme_map = {
        "MEV / Proposer-Builder Separation": [
            "mev", "builder", "relayer", "proposer-builder", "pbs", "inclusion", "searcher"
        ],
        "Blob Tx / Latency Issues": [
            "blob", "latency", "delay", "inclusion time", "eip-4844", "blobs", "propagation"
        ],
        "Gas Efficiency / Underutilization": [
            "gas limit", "calldata", "compression", "eip-2028", "underutilized", "gas usage", "efficiency"
        ],
        "State Bloat / Storage Costs": [
            "state bloat", "pruning", "storage rent", "storage cost", "eip-4444", "chaindata", "archive node"
        ],
        "Validator Incentive Design": [
            "reward", "incentive", "slashing", "proposer", "attester", "consensus reward", "eip-7002"
        ],
        "L1-L2 Bridge Issues": [
            "bridge", "rollup", "arbitrum", "optimism", "sequencer", "withdrawal delay", "l2", "message queue"
        ],
        "Security / DOS / Consensus Risks": [
            "consensus", "dos", "attack vector", "reorg", "fork", "safety", "finality", "spam", "consensus-breaking"
        ],
        "UX / Account Abstraction": [
            "smart account", "account abstraction", "eip-4337", "signer", "key rotation", "wallet UX", "meta tx"
        ],
        "Network Congestion / Mempool": [
            "mempool", "congestion", "pending tx", "broadcast", "spam", "txpool", "backlog"
        ],
        "Fee Market / Economic Design": [
            "eip-1559", "basefee", "tip", "priority fee", "fee market", "surge pricing", "burn", "tx pricing"
        ],
        "Backward Compatibility / Breaking Changes": [
            "breaking change", "incompatible", "legacy", "client crash", "hard fork", "backward compatibility"
        ],
        "Governance / Human Alignment": [
            "community concern", "developer disagreement", "governance", "eth ethos", "alignment", "values"
        ]
    }

    output = []
    seen = set()

    for doc in state.similar_docs:
        doc_id = f"{doc.metadata.get('source', '')}::{doc.metadata.get('title', '')}"
        if doc_id in seen:
            continue
        seen.add(doc_id)

        text = doc.page_content.lower()
        for theme, keywords in theme_map.items():
            if any(kw in text for kw in keywords):
                output.append({
                    "theme": theme,
                    "source": doc.metadata.get("url", "unknown"),
                    "eip": doc.metadata.get("eip", "unknown"),
                    "section": doc.metadata.get("section", "unknown"),
                    "content": doc.page_content,
                    "title": doc.metadata.get("title", "untitled"),
                    "severity": 0.8
                })

    return AgentState(friction_themes=output, **state.model_dump(exclude={"friction_themes"}))

# ---------- Step 6: Store Output ----------
def store_node(state):
    path = "friction_output.jsonl"
    with open(path, "w") as f:
        for entry in state.friction_themes:
            f.write(json.dumps(entry) + "\n")
    return AgentState(status="stored", path=path, **state.model_dump(exclude={"status", "path"}))


# ---------- Step 7: Proposal Engine (Model fine tuning) -------------
"""
Train a Change Generator (Policy Model)

This node acts as a proposal engine. Given an identified friction or theme (e.g. “MEV delays block inclusion”), it performs the following:

1. Retrieves historical EIPs that attempted to solve similar problems
2. Synthesizes a new candidate proposal inspired by previous solutions

3. Returns a draft proposal including:
   - Title
   - Summary
   - Motivation
   - Specification
   - Rationale
"""
def generate_proposal_node(state):
    from langchain_core.prompts import PromptTemplate
    from langchain_openai import ChatOpenAI
    from langchain.chains import LLMChain

    llm = ChatOpenAI(temperature=0.7, model="gpt-4") 

    prompt = PromptTemplate.from_template("""
    Given the following protocol friction in Ethereum:

    ---
    {theme_summary}
    ---

    Write a draft proposal to address it.
    Your output should include:
    - Title
    - Summary
    - Motivation
    - Specification
    - Rationale

    Be inspired by the style of EIPs.
    """)

    theme_text = "\n".join(
        [f"{t['title']} — {t['theme']}" for t in state.friction_themes]
    )

    chain = LLMChain(llm=llm, prompt=prompt)
    result = chain.run({"theme_summary": theme_text})

    return AgentState(proposal_draft=result, **state.model_dump(exclude={"proposal_draft"}))

# ---------- Step 8: Build Graph ----------
graph = StateGraph(state_schema=AgentState)
graph.add_node("Preprocess", preprocess_node)
graph.add_node("Embed", embed_node)
graph.add_node("RetrieveSimilar", retrieve_node)
graph.add_node("Classify", classify_node)
graph.add_node("GenerateProposal", generate_proposal_node)
graph.add_node("Store", store_node)

graph.set_entry_point("Preprocess")
graph.add_edge("Preprocess", "Embed")
graph.add_edge("Embed", "RetrieveSimilar")
graph.add_edge("RetrieveSimilar", "Classify")
graph.add_edge("Classify", "GenerateProposal")
graph.add_edge("GenerateProposal", "Store")

runnable = graph.compile()


# ---------- Step 9: Run Agent ----------
def serialize_output(raw_output):
    state = AgentState(**raw_output)
    return json.dumps(
        state.model_dump(exclude={"vectorstore", "documents"}),  # don't dump big or unserializable stuff
        indent=2
    )

if __name__ == "__main__":
    ingest_core_eips("core_eips.jsonl", limit=500)

    final_state = AgentState(**runnable.invoke(AgentState(documents=load_eip_documents("core_eips.jsonl"))))

    print("\n===== Proposal Draft =====\n")
    print(final_state.proposal_draft)

    print("\n===== Full Output =====\n")
    print(json.dumps(final_state.model_dump(exclude={"vectorstore", "documents"}), indent=2))




