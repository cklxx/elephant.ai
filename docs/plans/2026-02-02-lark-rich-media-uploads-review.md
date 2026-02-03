# Plan: Inspect Lark rich media cards + file/image uploads

## Goals
- Locate Lark message send paths for interactive cards, rich text posts, and file/image uploads.
- Identify integration points/APIs used (gateway, messenger, SDK calls, attachment pipeline).
- Summarize findings and suggest best approach for extending/using these capabilities.

## Steps
1. Scan Lark gateway/messenger/card/richcontent code to map message types and upload flow.
2. Trace attachment pipeline (auto-upload config, attachment resolver, media typing) to understand upload triggers.
3. Summarize integration points/APIs and propose recommended usage/extension approach.

## Status
- [x] Step 1
- [x] Step 2
- [x] Step 3
