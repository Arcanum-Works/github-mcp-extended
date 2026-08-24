# Project instructions: github-mcp-extended

## `.env` access and credentials

You are explicitly authorized to read from and write to the local `.env` file when required to complete or verify this repository's work.

If the required `.env` file does not exist:

- inspect `.env.example`, `.env.sample`, documentation, and code to determine the expected variables
- create the local `.env` if appropriate
- populate values that can be derived safely from the existing local environment/configuration
- if a required secret or credential is genuinely unavailable, stop and tell me exactly which variable is missing

If you cannot access or modify `.env` because of filesystem permissions:

- inspect the ownership and permissions first
- make the smallest safe local permission/ownership adjustment necessary to allow the current development user to read/write it
- do not broadly weaken permissions such as `chmod 777`
- do not alter unrelated files or system-wide security settings

You may update existing `.env` values when necessary for the task.

### Secret-safety rules

- Never commit `.env` or credentials to Git.
- Confirm `.env` is ignored before placing secrets in it; add the appropriate ignore rule if necessary.
- Never paste secret values, API keys, tokens, passwords, or full credential contents into chat, logs, PR descriptions, commits, or the final report.
- When reporting configuration, mention only variable names and whether they are present/configured.
- Do not overwrite a working credential unnecessarily.
- Before replacing an existing `.env` value, preserve the existing configuration locally or make the change reversible.
- Do not modify billing, rotate credentials, revoke tokens, or create new paid credentials unless explicitly authorized separately.

Treat the local `.env` as available development configuration, not as a blocker merely because an earlier agent could not access or write it.
