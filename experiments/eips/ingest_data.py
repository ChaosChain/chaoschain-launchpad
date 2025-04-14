import requests
from bs4 import BeautifulSoup
import json
import time

def extract_eip_sections(soup):
    content_div = soup.find("div", {"class": "home"})
    if not content_div:
        return {}

    sections = {}
    tags = list(content_div.children)

    for i, tag in enumerate(tags):
        if tag.name == "h2" or tag.name == "h1" or tag.name == "h3":
            section_title = tag.get_text(strip=True)
            # Find the next <p> tag after the <h2>
            for next_tag in tags[i+1:]:
                if next_tag.name == "p":
                    section_content = next_tag.get_text(strip=True)
                    sections[section_title] = section_content
                    break

    return sections


def ingest_core_eips(output_path="core_eips.jsonl", limit=None):
    base_list_url = "https://eips.ethereum.org/core"
    base_eip_url = "https://eips.ethereum.org/EIPS/eip-"

    res = requests.get(base_list_url)
    soup = BeautifulSoup(res.text, "html.parser")

    all_tables = soup.find_all("table", class_="eiptable")
    seen = set()
    entries = []

    for table in all_tables:
        rows = table.find_all("tr")

        for row in rows:
            if limit and len(entries) >= limit:
                break

            cols = row.find_all("td")
            if len(cols) < 3:
                continue

            eip_link = cols[0].find("a")
            if not eip_link:
                continue

            try:
                eip_number = int(eip_link.text.strip())
            except:
                continue

            if eip_number in seen:
                continue
            seen.add(eip_number)

            title = cols[1].text.strip()
            author_text = cols[2].get_text(separator=",").strip()
            authors = [a.strip() for a in author_text.split(",")]

            eip_url = f"{base_eip_url}{eip_number}"
            try:
                eip_res = requests.get(eip_url)
                eip_soup = BeautifulSoup(eip_res.text, "html.parser")
                sections = extract_eip_sections(eip_soup)

                data = {
                    "eip": eip_number,
                    "url": eip_url,
                    "title": title,
                    "authors": authors,
                    "sections": sections,
                    "content_type": "core-eip"
                }

                entries.append(data)
                time.sleep(0.25)

            except Exception as e:
                print(f"Failed to fetch EIP-{eip_number}: {e}")

    with open(output_path, "w") as f:
        for entry in entries:
            f.write(json.dumps(entry) + "\n")

    print(f"Ingested {len(entries)} core EIPs → {output_path}")
    return entries
