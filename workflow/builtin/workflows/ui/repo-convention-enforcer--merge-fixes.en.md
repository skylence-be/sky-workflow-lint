---
short_description: Merges the approved cleanup.
long_description: Opens a pull request for the approved style fixes (if one doesn't already exist) and merges it automatically. Does nothing if there was nothing to fix.
---
Once the cleanup is approved, this step wraps things up: it opens a pull request if there isn't one already, and merges it in automatically. If there was nothing to fix in the first place, it simply stops without doing anything.
