# Remaining evaluation features to support

- **Realtime web updates**: push job status and worker progress via SSE/WebSockets to avoid manual refreshes.
- **Export and sharing**: allow downloading evaluation reports (Markdown/JSON) and exposing shareable permalinks for completed jobs.
- **Access control and quotas**: enforce per-user authentication, rate limits, and retention policies for stored evaluations and profiles.
- **Search and filtering**: broaden beyond current agent/time/score/dataset/tag filters to support richer fields (owners, free-text query) and cross-agent catalog search.
- **Retention and cleanup**: add APIs/CLI knobs to prune old evaluations or expired profiles automatically.
