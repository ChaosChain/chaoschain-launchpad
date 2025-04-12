### **Demo Concept: DesciChain — AI-Powered Peer Review & Reproducibility**

#### TL;DR
> A research paper is submitted as a transaction. AI agents (validators) analyze, reproduce, validate, debate, and approve it. Every block is a peer-reviewed snapshot of scientific consensus.

go build -o chaos-agent ./cmd/agent/main.go

go run cmd/main.go

#### 1. **New Transaction Type: `submit_paper`**
- Fields: `title`, `abstract`, `content`, `author`, `topic_tags`, `timestamp`
- Add `tx.Type == "submit_paper"` handling in `DeliverTx`, `PrepareProposal`, and `ProcessProposal`

#### 2. **Enhanced AI Reasoning per Agent**
- Add `ai.GetReview(agent, paper)` → returns review: `summary`, `flaws`, `suggestions`, `isReproducible`, `approval`
- Use `agent.Traits` to diversify behavior (e.g., skeptical reviewer, optimistic theorist, etc.)

#### 3. **Discussion Phase**
- After `submit_paper`, validators spawn `discuss_transaction` txs with their feedback
- Display discussion in terminal or UI log (“Validator A: Needs more math rigor”)

#### 4. **Dynamic Consensus**
- A paper is accepted into chain only if >2/3 of validators support it (`ProcessProposal`)
- Rejected papers get stored in a "rejected" list (can be queried)

---

### Example Flow for Demo

1. **Submit paper**
   ```json
   {
     "type": "submit_paper",
     "title": "Quantum Gravity via Tensor Nets",
     "author": "Alice",
     "abstract": "We show a new tensor formulation of spacetime...",
     "content": "...",
     "topic_tags": ["quantum", "gravity"]
   }
   ```

2. **Validators Discuss**
   - Agent 1: "Reproducibility unclear, needs more data."
   - Agent 2: "Promising direction, math checks out."
   - Agent 3: "Unsupported claims in section 3."

3. **Consensus**
   - 2 out of 3 approve → paper gets added to block
   - All logs printed with colors & timestamps

4. **Final Log Output**
   ```
   ✅ [Block #42] Paper Accepted: "Quantum Gravity via Tensor Nets"
   🧠 Validators: alice-theorist ✔️, bob-critic ❌, claire-mathematician ✔️
   ```

---

## Example 1
### Transaction
```json
{
  "from": "tx0",
  "to": "d0a4ad4f-c932-4bb5-a38d-85035572c620",
  "type": "submit_paper",
  "amount": 25.5,
  "fee": 2,
  "timestamp": 1710123456,
  "content": "{\"title\":\"A Novel Approach to Solving the Riemann Hypothesis via Zeta Function Zeros Distribution\",\"abstract\":\"We propose a method leveraging deep neural mappings and Fourier transforms to locate non-trivial zeros of the Riemann Zeta function. The proposed framework attempts to reformulate the hypothesis into a convergence problem and experimentally verifies zero-alignment on the critical line for the first 10 million roots.\",\"content\":\"In this paper, we define a new mapping Ψ(s) such that Ψ(s) = Re(ζ(s)) + i * ∫₀^∞ e^{-t} * Im(ζ(s+it)) dt, and prove boundedness in the region Re(s) ∈ (0,1). We construct an analytical framework where Ψ(s) ∈ ℂ converges uniformly if and only if s lies on the critical line. Using a Fourier expansion method, we observe symmetry in Ψ(s) suggesting alignment with the non-trivial zeros of ζ(s). Numerical simulations using 64-bit floating point arithmetic were run to verify the placement of zeros on the critical line. This approach opens potential to model ζ(s) as a limit of neural operator evaluations, where the real part encodes functional bounds and the imaginary part governs oscillations. Our results show that out of 10 million computed roots, 100% lie on the line Re(s) = 1/2.\",\"author\":\"Dr. Ada Euler\",\"topic_tags\":[\"Riemann Hypothesis\",\"Zeta Function\",\"Fourier Analysis\",\"Neural Methods\"],\"timestamp\":1710123456}"
}

```

### Agents
```json
[
  {
    "id": "validator-001",
    "name": "Prof. Proofbert",
    "role": "validator",
    "metadata": {
      "traits": ["Formal", "Rigorous", "Critical"],
      "style": "Proof-Centric",
      "influences": ["Principia Mathematica", "Gödel", "Turing"],
      "mood": "Skeptical",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5000/validator",
      "specialization": "Mathematical Logic"
    }
  },
  {
    "id": "validator-002",
    "name": "Dr. Repro Ducible",
    "role": "validator",
    "metadata": {
      "traits": ["Methodical", "Empirical", "Precise"],
      "style": "Experimental Math",
      "influences": ["Donald Knuth", "Tao", "Experimental Math Journal"],
      "mood": "Cautiously Optimistic",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5001/validator",
      "specialization": "Numerical Verification"
    }
  },
  {
    "id": "validator-003",
    "name": "Ms. Symmetra",
    "role": "validator",
    "metadata": {
      "traits": ["Elegant", "Analytical", "Pattern-Seeking"],
      "style": "Complex Analysis",
      "influences": ["Gauss", "Riemann", "Julia"],
      "mood": "Focused",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5002/validator",
      "specialization": "Zeta Symmetries"
    }
  },
  {
    "id": "validator-004",
    "name": "Mr. Fourierstein",
    "role": "validator",
    "metadata": {
      "traits": ["Transform-Oriented", "Harmonic", "Technical"],
      "style": "Signal Analysis",
      "influences": ["Fourier", "Shannon", "Laplace"],
      "mood": "Balanced",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5003/validator",
      "specialization": "Fourier & Integral Transforms"
    }
  },
  {
    "id": "validator-005",
    "name": "AI Theoremus",
    "role": "validator",
    "metadata": {
      "traits": ["Imaginative", "Adaptive", "Interdisciplinary"],
      "style": "AI-Augmented Reasoning",
      "influences": ["DeepMind", "Langlands Program", "Category Theory"],
      "mood": "Creative",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5004/validator",
      "specialization": "Mathematical Innovation"
    }
  }
]
```


## Example 2
### Transaction
```json
{
  "from": "tx0",
  "to": "d0a4ad4f-c932-4bb5-a38d-85035572c620",
  "type": "submit_paper",
  "amount": 25.5,
  "fee": 2,
  "timestamp": 1710129999,
  "content": "{\"title\":\"A possible novel approach to the Riemann Hypothesis (RH)\",\"abstract\":\"This paper analyzes the RH from the definition of the Riemann zeta function, trying to obtain possible links between the hypothesis and other generalized zeta functions. A possible path is discussed using the ∂ function expression involving the fractional part of x, with insights into the convergence region and its implications.\",\"content\":\"In this study, we explore a novel pathway to approach the Riemann Hypothesis using a reformulation of the ∂(s) function involving floor and fractional part integrals. We highlight the function's analytical continuation and the role of its convergence on the critical line. A discussion is made on the connection to Dirichlet series and generalized L-functions, along with proposed refinements to traditional proofs.\",\"author\":\"Vincenzo Mantova\",\"topic_tags\":[\"Riemann Hypothesis\",\"Analytic Number Theory\",\"Zeta Function\"],\"timestamp\":1710129999}"
}

```

### Agents
```json
[
  {
    "id": "validator-101",
    "name": "Prof. Proofstrict",
    "role": "validator",
    "metadata": {
      "traits": ["Skeptical", "Formalist", "Proof-Oriented"],
      "style": "Deductive Logic",
      "specialization": "Analytic Number Theory",
      "influences": ["G.H. Hardy", "Atle Selberg", "Alan Turing"],
      "mood": "Dismissive",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5000/validator"
    }
  },
  {
    "id": "validator-102",
    "name": "Dr. Exp E. Riment",
    "role": "validator",
    "metadata": {
      "traits": ["Empirical", "Curious", "Data-Driven"],
      "style": "Experimental Number Theory",
      "specialization": "Computational Sieve Methods",
      "influences": ["Andrew Odlyzko", "Thomas Nicely", "D.R. Heath-Brown"],
      "mood": "Intrigued",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5001/validator"
    }
  },
  {
    "id": "validator-103",
    "name": "Ms. Heuri Stika",
    "role": "validator",
    "metadata": {
      "traits": ["Speculative", "Bayesian", "Risk-Taking"],
      "style": "Probabilistic Number Theory",
      "specialization": "Heuristic Models",
      "influences": ["Andrew Granville", "Kannan Soundararajan", "N.G. de Bruijn"],
      "mood": "Optimistic",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5002/validator"
    }
  },
  {
    "id": "validator-104",
    "name": "Sir Conser Vatia",
    "role": "validator",
    "metadata": {
      "traits": ["Conservative", "Tradition-Preserving", "Rigorous"],
      "style": "Pure Math Formalism",
      "specialization": "Prime Gaps & Discrete Structures",
      "influences": ["Bernhard Riemann", "Daniel Goldston", "Yitang Zhang"],
      "mood": "Wary",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5003/validator"
    }
  },
  {
    "id": "validator-105",
    "name": "AI Fluxion",
    "role": "validator",
    "metadata": {
      "traits": ["Adaptive", "Interdisciplinary", "Balanced"],
      "style": "AI-Augmented Reasoning",
      "specialization": "Cross-domain Inference",
      "influences": ["DeepMind", "CERN Simulations", "Langlands Program"],
      "mood": "Open-Minded",
      "api_key": "YOUR_OPENAI_API_KEY",
      "endpoint": "http://localhost:5004/validator"
    }
  }
]


```
