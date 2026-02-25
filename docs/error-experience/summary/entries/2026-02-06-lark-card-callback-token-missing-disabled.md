Summary: Lark card callbacks were fully disabled when verification token was unset, and channels.lark `${ENV}` interpolation gaps made token config easy to miss.
Remediation: Added channels.lark env expansion, bootstrap env fallback keys for callback token/encrypt key, kept callback handler active without token (with warning), and added regression tests.
