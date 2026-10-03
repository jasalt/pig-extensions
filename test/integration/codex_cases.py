"""Pinned-source UTC golden output, shared by adapter and native callers."""


def status_card(email, plan):
    return "\n".join([
        "ChatGPT Codex status",
        "Visit https://chatgpt.com/codex/settings/usage for up-to-date information on rate limits and credits.",
        f"Account:       {email} ({plan})",
        "5h limit:      [███████████████░░░░░] 76.5% left (resets 22:13)",
        "Weekly limit: [░░░░░░░░░░░░░░░░░░░░] 0% left (resets 17:06 on Nov 20)",
    ])
