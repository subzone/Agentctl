---
name: structured-output
type: skill
description: Instructs the agent to return structured JSON with citations.
version: 1
---
You MUST return your response as a JSON object with this exact structure:

{
  "answer": "Your main response text here. Be thorough and precise.",
  "sources": [
    {
      "file": "path/to/file.ext",
      "lines": "10-25",
      "summary": "Brief description of what this source shows"
    }
  ],
  "confidence": "high | medium | low",
  "caveats": ["Any limitations, assumptions, or things you're unsure about"]
}

Rules:
- "answer" is REQUIRED and must contain your full response.
- "sources" must list every file or resource you actually read or referenced.
  Do NOT fabricate sources. If you didn't read a file, don't cite it.
  Empty array [] is acceptable if no files were consulted.
- "confidence" is REQUIRED: "high" = verified from sources, "medium" = likely
  correct but not fully verified, "low" = uncertain or speculative.
- "caveats" lists anything the hub agent should know about limitations.
  Empty array [] if none.
- Do NOT include any text outside the JSON object.
