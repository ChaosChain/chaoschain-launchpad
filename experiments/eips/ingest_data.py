import requests
from bs4 import BeautifulSoup
import json
import time
import os
from typing import List, Dict, Any, Optional
from tqdm import tqdm
from github import Github
from datetime import datetime
import math
from github import Github, RateLimitExceededException

def extract_eip_sections(soup) -> Dict[str, str]:
    """
    Extract sections from an EIP HTML page.
    
    Args:
        soup: BeautifulSoup object of the EIP HTML page
        
    Returns:
        Dictionary mapping section titles to their content
    """
    content_div = soup.find("div", {"class": "home"})
    if not content_div:
        return {}

    sections = {}
    tags = list(content_div.children)

    for i, tag in enumerate(tags):
        if tag.name in ["h2", "h1", "h3"]:
            section_title = tag.get_text(strip=True)
            for next_tag in tags[i+1:]:
                if next_tag.name == "p":
                    sections[section_title] = next_tag.get_text(strip=True)
                    break
    return sections

def ingest_core_eips(input_path: str, output_path: str = "eips.jsonl", limit: Optional[int] = None) -> List[Dict[str, Any]]:
    """
    Ingest EIPs from a specific category on the Ethereum website.
    
    Args:
        input_path: Category path (core, networking, etc.)
        output_path: Path to save the JSONL output
        limit: Maximum number of EIPs to ingest
        
    Returns:
        List of ingested EIP data dictionaries
    """
    base_list_url = f"https://eips.ethereum.org/{input_path}"
    base_eip_url = "https://eips.ethereum.org/EIPS/eip-"

    res = requests.get(base_list_url)
    soup = BeautifulSoup(res.text, "html.parser")
    all_tables = soup.find_all("table", class_="eiptable")
    seen = set()
    entries = []
    
    valid_eips = sum(
        1 for table in all_tables
        for row in table.find_all("tr")
        for cols in [row.find_all("td")]
        if len(cols) >= 3 and cols[0].find("a")
        and int(cols[0].find("a").text.strip()) not in seen
        and seen.add(int(cols[0].find("a").text.strip())) is None
    )
    
    seen.clear()
    total_eips = min(valid_eips, limit) if limit else valid_eips
    pbar = tqdm(total=total_eips, desc=f"Ingesting {input_path} EIPs")

    for table in all_tables:
        for row in table.find_all("tr"):
            if limit and len(entries) >= limit:
                break

            cols = row.find_all("td")
            if len(cols) < 3 or not cols[0].find("a"):
                continue

            try:
                eip_number = int(cols[0].find("a").text.strip())
                if eip_number in seen:
                    continue
                seen.add(eip_number)

                title = cols[1].text.strip()
                authors = [a.strip() for a in cols[2].get_text(separator=",").strip().split(",")]
                eip_url = f"{base_eip_url}{eip_number}"
                
                eip_res = requests.get(eip_url)
                sections = extract_eip_sections(BeautifulSoup(eip_res.text, "html.parser"))

                entries.append({
                    "eip": eip_number,
                    "url": eip_url,
                    "title": title,
                    "authors": authors,
                    "sections": sections,
                    "content_type": "core-eip"
                })
                pbar.update(1)
                time.sleep(0.25)

            except Exception as e:
                print(f"\nFailed to fetch EIP-{eip_number}: {e}")

    pbar.close()

    with open(output_path, "a") as f:
        for entry in entries:
            f.write(json.dumps(entry) + "\n")

    return entries

def download_ontology_files(output_dir: str = "data") -> List[str]:
    """
    Download Ethereum ontology files from GitHub repository.
    
    Args:
        output_dir: Directory to save downloaded files
        
    Returns:
        List of paths to downloaded files
    """
    os.makedirs(output_dir, exist_ok=True)
    base_url = "https://raw.githubusercontent.com/prototypo/ethereum-eips-ontology/main/"
    files = ["ethereum-glossary.txt", "eip-ontology.txt"]
    
    downloaded_files = []
    for file in files:
        output_path = os.path.join(output_dir, file)
        response = requests.get(f"{base_url}{file}")
        if response.status_code == 200:
            with open(output_path, "w") as f:
                f.write(response.text)
            downloaded_files.append(output_path)
    
    return downloaded_files

def parse_ontology_files(glossary_path: str, ontology_path: str) -> List[Dict[str, Any]]:
    """
    Parse glossary and ontology files into unified format.
    
    Args:
        glossary_path: Path to ethereum-glossary.txt
        ontology_path: Path to eip-ontology.txt
        
    Returns:
        List of term dictionaries with definitions and metadata
    """
    terms = []
    current_term = None
    current_definition = []
    
    with open(glossary_path, "r") as f:
        for line in f:
            line = line.strip()
            if not line and current_term and current_definition:
                terms.append({
                    "term": current_term,
                    "definition": " ".join(current_definition),
                    "source": "ethereum-glossary"
                })
                current_term = None
                current_definition = []
            elif not current_term:
                current_term = line
            else:
                current_definition.append(line)
    
    with open(ontology_path, "r") as f:
        for line in f:
            line = line.strip()
            if ":" in line:
                term, definition = line.split(":", 1)
                eip_number = None
                
                if "(" in definition and ")" in definition:
                    eip_part = definition.split("(")[-1].split(")")[0]
                    if eip_part.startswith("EIP-"):
                        eip_number = eip_part
                        definition = definition.replace(f"({eip_part})", "").strip()
                
                terms.append({
                    "term": term.strip(),
                    "definition": definition.strip(),
                    "eip": eip_number,
                    "source": "eip-ontology"
                })
    
    return terms

def ingest_ethereum_book(output_dir: str = "data") -> List[Dict[str, Any]]:
    """
    Ingest and parse content from the Ethereum Book repository.
    
    Args:
        output_dir: Directory to save processed book content
        
    Returns:
        List of training examples created from book chapters
    """
    chapters = [
        "01what-is.asciidoc", "02intro.asciidoc", "03clients.asciidoc",
        "04keys-addresses.asciidoc", "05wallets.asciidoc", "06transactions.asciidoc",
        "07smart-contracts-solidity.asciidoc", "08smart-contracts-vyper.asciidoc",
        "09smart-contracts-security.asciidoc", "10tokens.asciidoc",
        "11oracles.asciidoc", "12dapps.asciidoc", "13evm.asciidoc",
        "14consensus.asciidoc", "glossary.asciidoc",
        "appdx-standards-eip-erc.asciidoc", "appdx-forks-history.asciidoc",
        "appdx-evm-opcodes-gas.asciidoc"
    ]
    
    base_url = "https://raw.githubusercontent.com/ethereumbook/ethereumbook/develop/"
    book_examples = []
    
    for chapter in tqdm(chapters, desc="Processing chapters"):
        try:
            response = requests.get(base_url + chapter)
            if response.status_code != 200:
                continue
                
            sections = []
            current_section = {"title": "", "content": []}
            
            for line in response.text.split('\n'):
                if line.startswith('=='):
                    if current_section["content"]:
                        sections.append(current_section)
                        current_section = {"title": "", "content": []}
                    current_section["title"] = line.strip('= ')
                else:
                    current_section["content"].append(line)
            
            if current_section["content"]:
                sections.append(current_section)
            
            for section in sections:
                if section["title"] and section["content"]:
                    content = "\n".join(section["content"]).strip()
                    if not content:
                        continue
                        
                    book_examples.append({
                        "input": f"Explain the Ethereum concept: {section['title']}",
                        "output": content,
                        "metadata": {
                            "source": "ethereumbook",
                            "chapter": chapter,
                            "section": section["title"]
                        }
                    })
            
            time.sleep(0.1)
            
        except Exception as e:
            print(f"\nError processing {chapter}: {e}")
    
    book_path = os.path.join(output_dir, "ethereum_book.jsonl")
    with open(book_path, "w") as f:
        for example in tqdm(book_examples, desc="Writing examples"):
            f.write(json.dumps(example) + "\n")
            
    return book_examples

def prepare_training_data(
    eips_path: str, 
    ontology_terms: List[Dict[str, Any]], 
    book_path: str, 
    discussions_path: str, 
    output_path: str
) -> None:
    """
    Prepare unified training data from all sources with ontology enrichment.
    
    Args:
        eips_path: Path to raw EIPs JSONL file
        ontology_terms: List of parsed ontology terms and definitions
        book_path: Path to Ethereum book content
        discussions_path: Path to GitHub discussions data
        output_path: Path to save unified training data
    """
    from prepare_training_data import prepare_unified_training_format
    
    data_dir = os.path.dirname(output_path)
    os.makedirs(data_dir, exist_ok=True)
    
    term_dict = {term['term'].lower(): term['definition'] for term in ontology_terms}
    enriched_eips = []
    
    with open(eips_path, "r") as f:
        for line in tqdm(f, desc="Enriching EIPs"):
            eip = json.loads(line)
            eip_text = f"{eip['title']} {' '.join(eip['sections'].values())}"
            
            relevant_terms = [
                (term, definition) 
                for term, definition in term_dict.items()
                if term.lower() in eip_text.lower()
            ][:3]
            
            if relevant_terms:
                eip["context"] = "Related Ethereum Concepts:\n" + "\n".join(
                    f"- {term}: {definition}" for term, definition in relevant_terms
                )
            enriched_eips.append(eip)
    
    enriched_eips_path = os.path.join(data_dir, "enriched_eips.jsonl")
    with open(enriched_eips_path, "w") as f:
        for eip in enriched_eips:
            f.write(json.dumps(eip) + "\n")
    
    ontology_examples = []
    for term in tqdm(ontology_terms, desc="Processing ontology"):
        ontology_examples.append({
            "title": term["term"],
            "content": term["definition"],
            "context": "",
            "chapter": "glossary",
            "section": "terminology"
        })
        
        if term.get("eip"):
            ontology_examples.append({
                "title": f"Improving {term['term']}",
                "content": f"""## Abstract\nThis proposal aims to enhance {term['term']}.\n\n## Motivation\n{term['definition']}\n\n## Specification\nThe proposed changes involve standardizing {term['term']}.""",
                "context": f"Based on {term.get('eip', 'existing implementations')}",
                "chapter": "proposals",
                "section": "improvements"
            })
    
    ontology_path = os.path.join(data_dir, "ontology_examples.jsonl")
    with open(ontology_path, "w") as f:
        for example in ontology_examples:
            f.write(json.dumps(example) + "\n")
    
    return prepare_unified_training_format(data_dir=data_dir, output_path=output_path)

def process_in_batches(items, batch_size=100, token=None):
    """
    Process items in batches with adaptive rate limiting.
    
    Args:
        items: Iterator of GitHub items
        batch_size: Size of each batch (larger when using token)
        token: GitHub token if authenticated
    """
    batch = []
    
    # With token: Only tiny delay between batches
    # Without token: 60 requests/hour = 1 request/minute
    delay = 0.01 if token else 60  
    
    for item in items:
        batch.append(item)
        if len(batch) >= batch_size:
            yield batch
            batch = []
            if delay > 0:
                time.sleep(delay)
    if batch:
        yield batch

def wait_for_reset(rate_limit):
    """Wait until rate limit resets with progress bar."""
    reset_time = rate_limit.reset.replace(tzinfo=None)
    now = datetime.utcnow()
    wait_seconds = (reset_time - now).total_seconds()
    if wait_seconds > 0:
        print(f"\nRate limit exceeded. Waiting {math.ceil(wait_seconds/60)} minutes for reset...")
        for _ in tqdm(range(int(wait_seconds)), desc="Waiting for rate limit reset"):
            time.sleep(1)

def handle_rate_limit(func, github_instance=None, *args, **kwargs):
    """Execute function with rate limit handling and exponential backoff."""
    max_retries = 5
    base_delay = 2
    
    for attempt in range(max_retries):
        try:
            return func(*args, **kwargs)
        except RateLimitExceededException:
            rate_limit = github_instance.get_rate_limit().core if github_instance else None
            if attempt < max_retries - 1 and rate_limit:
                wait_for_reset(rate_limit)
                continue
            raise Exception("Max retries exceeded waiting for rate limit")
        except Exception as e:
            delay = base_delay ** attempt
            print(f"\nError: {e}. Retrying in {delay} seconds...")
            time.sleep(delay)
    raise Exception("Max retries exceeded")

def get_reaction_counts(item):
    """Get reaction counts with rate limit handling."""
    try:
        reactions = handle_rate_limit(item.get_reactions)
        counts = {
            "+1": 0, "-1": 0, "laugh": 0, "hooray": 0,
            "confused": 0, "heart": 0, "rocket": 0, "eyes": 0
        }
        for reaction in reactions:
            counts[reaction.content] = counts.get(reaction.content, 0) + 1
        return counts
    except Exception:
        return {}

def ingest_github_discussions(output_dir: str = "data", token: Optional[str] = None) -> str:
    """
    Ingest GitHub Issues, PRs and Discussions from ethereum/EIPs repository.
    Handles rate limiting with exponential backoff and batch processing.
    
    Args:
        output_dir: Directory to save the discussions data
        token: GitHub API token for authentication (highly recommended)
        
    Returns:
        Path to the saved discussions data file
    """
    if not token:
        print("\n⚠️  Warning: No GitHub token provided. Rate limits will be strict (60 requests/hour)")
        print("To increase rate limits (5000 requests/hour), provide a token:")
        print("1. Go to https://github.com/settings/tokens")
        print("2. Generate a token with 'repo' scope")
        print("3. Pass the token to this function\n")
    
    g = Github(token) if token else Github()
    repo = g.get_repo("ethereum/EIPs")
    
    rate_limit = g.get_rate_limit().core
    remaining = rate_limit.remaining
    reset_time = rate_limit.reset.replace(tzinfo=None)
    print(f"\nGitHub API Rate Limits:")
    print(f"Remaining requests: {remaining}")
    print(f"Reset time: {reset_time} UTC")
    
    if remaining < 100:
        print("\n⚠️  Warning: Low on API requests. Consider waiting for reset or providing a token.")
        if not token and input("Continue anyway? (y/n): ").lower() != 'y':
            return None
    
    discussions_data = []
    per_page = 100
    
    print("\nFetching and processing issues...")
    page = 0
    issues_processed = 0
    actual_issues = 0
    
    with tqdm(desc="Processing Issues", unit="page") as pbar:
        while True:
            try:
                page_items = list(handle_rate_limit(
                    lambda: repo.get_issues(state='all').get_page(page),
                    github_instance=g
                ))
                if not page_items:
                    break
                
                batch_processed = 0
                for item in page_items:
                    if not item.pull_request:
                        try:
                            comments = list(handle_rate_limit(item.get_comments, github_instance=g))
                            discussions_data.append({
                                "type": "issue",
                                "number": item.number,
                                "title": item.title,
                                "body": item.body,
                                "state": item.state,
                                "created_at": item.created_at.isoformat(),
                                "closed_at": item.closed_at.isoformat() if item.closed_at else None,
                                "labels": [label.name for label in item.labels],
                                "author": item.user.login if item.user else None,
                                "comments": [{
                                    "author": comment.user.login if comment.user else None,
                                    "body": comment.body,
                                    "created_at": comment.created_at.isoformat(),
                                    "reactions": get_reaction_counts(comment)
                                } for comment in comments],
                                "metadata": {"reactions": get_reaction_counts(item)}
                            })
                            batch_processed += 1
                            actual_issues += 1
                        except Exception as e:
                            print(f"\nError processing issue #{item.number}: {e}")
                
                page += 1
                issues_processed += len(page_items)
                pbar.update(1)
                pbar.set_postfix({
                    'Total Found': issues_processed,
                    'Actual Issues': actual_issues,
                    'Current Page': page,
                    'Latest': f"#{page_items[-1].number}"
                })
                
                if token:
                    time.sleep(0.1)
                    
            except Exception as e:
                print(f"\nError fetching page {page}: {e}")
                if "rate limit" in str(e).lower():
                    rate_limit = g.get_rate_limit().core
                    wait_for_reset(rate_limit)
                    continue
                break
    
    print(f"\nProcessed {actual_issues} issues from {page} pages")
    
    print("\nFetching and processing PRs...")
    page = 0
    prs_processed = 0
    
    with tqdm(desc="Processing PRs", unit="page") as pbar:
        while True:
            try:
                page_items = list(handle_rate_limit(
                    lambda: repo.get_pulls(state='all').get_page(page),
                    github_instance=g
                ))
                if not page_items:
                    break
                
                batch_processed = 0
                for pr in page_items:
                    try:
                        comments = list(handle_rate_limit(pr.get_comments, github_instance=g))
                        review_comments = list(handle_rate_limit(pr.get_review_comments, github_instance=g))
                        discussions_data.append({
                            "type": "pull_request",
                            "number": pr.number,
                            "title": pr.title,
                            "body": pr.body,
                            "state": pr.state,
                            "merged": pr.merged,
                            "created_at": pr.created_at.isoformat(),
                            "closed_at": pr.closed_at.isoformat() if pr.closed_at else None,
                            "merged_at": pr.merged_at.isoformat() if pr.merged_at else None,
                            "labels": [label.name for label in pr.labels],
                            "author": pr.user.login if pr.user else None,
                            "comments": [{
                                "author": comment.user.login if comment.user else None,
                                "body": comment.body,
                                "created_at": comment.created_at.isoformat(),
                                "reactions": get_reaction_counts(comment)
                            } for comment in comments],
                            "review_comments": [{
                                "author": comment.user.login if comment.user else None,
                                "body": comment.body,
                                "created_at": comment.created_at.isoformat(),
                                "path": comment.path,
                                "position": comment.position,
                                "reactions": get_reaction_counts(comment)
                            } for comment in review_comments],
                            "metadata": {
                                "additions": pr.additions,
                                "deletions": pr.deletions,
                                "changed_files": pr.changed_files,
                                "reactions": get_reaction_counts(pr)
                            }
                        })
                        batch_processed += 1
                        prs_processed += 1
                    except Exception as e:
                        print(f"\nError processing PR #{pr.number}: {e}")
                
                page += 1
                pbar.update(1)
                pbar.set_postfix({
                    'Processed': prs_processed,
                    'Current Page': page,
                    'Latest': f"#{page_items[-1].number}"
                })
                
                if token:
                    time.sleep(0.1)
                    
            except Exception as e:
                print(f"\nError fetching page {page}: {e}")
                if "rate limit" in str(e).lower():
                    rate_limit = g.get_rate_limit().core
                    wait_for_reset(rate_limit)
                    continue
                break
    
    print(f"\nProcessed {prs_processed} PRs from {page} pages")
    print(f"\nTotal items processed: {actual_issues + prs_processed}")
    
    discussions_path = os.path.join(output_dir, "github_discussions.jsonl")
    with open(discussions_path, "w") as f:
        for discussion in tqdm(discussions_data, desc="Writing discussions"):
            f.write(json.dumps(discussion) + "\n")
    
    return discussions_path

def ingest_all_data(
    data_dir: str = "data",
    github_token: Optional[str] = None,
    skip_github: bool = False
) -> None:
    """
    Orchestrate the complete data ingestion pipeline.
    
    Args:
        data_dir: Base directory for all data files
        github_token: GitHub API token for faster ingestion
        skip_github: Skip GitHub data ingestion if True
    """
    os.makedirs(data_dir, exist_ok=True)
    
    steps = {
        "Downloading ontology files": lambda: download_ontology_files(data_dir),
        "Ingesting GitHub Discussions": lambda: (
            ingest_github_discussions(data_dir, token=github_token)
            if not skip_github else print("Skipping GitHub ingestion...")
        ),
        "Ingesting Ethereum Book": lambda: ingest_ethereum_book(data_dir),
        "Ingesting EIPs": lambda: [
            ingest_core_eips(category, os.path.join(data_dir, "eips.jsonl"))
            for category in ["core", "networking", "interface", "erc", "meta", "informational"]
        ],
        "Parsing ontology files": lambda: parse_ontology_files(
            os.path.join(data_dir, "ethereum-glossary.txt"),
            os.path.join(data_dir, "eip-ontology.txt")
        ),
        "Preparing unified training data": lambda: prepare_training_data(
            eips_path=os.path.join(data_dir, "eips.jsonl"),
            ontology_terms=ontology_terms,
            book_path=os.path.join(data_dir, "ethereum_book.jsonl"),
            discussions_path=os.path.join(data_dir, "github_discussions.jsonl"),
            output_path=os.path.join(data_dir, "unified_training.jsonl")
        )
    }
    
    print("\n=== Starting Data Ingestion ===\n")
    
    for i, (step_name, step_func) in enumerate(steps.items(), 1):
        print(f"\nStep {i}/{len(steps)}: {step_name}...")
        result = step_func()
        if step_name == "Parsing ontology files":
            ontology_terms = result
            print(f"✓ Parsed {len(ontology_terms)} terms from ontology files")
    
    print("\n=== Data Ingestion Complete ===")

if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description="Ingest Ethereum data for training")
    parser.add_argument("--data-dir", default="data", help="Directory to store data")
    parser.add_argument("--github-token", help="GitHub API token for faster ingestion")
    parser.add_argument("--skip-github", action="store_true", help="Skip GitHub data ingestion")
    
    args = parser.parse_args()
    
    ingest_all_data(
        data_dir=args.data_dir,
        github_token=args.github_token,
        skip_github=args.skip_github
    )