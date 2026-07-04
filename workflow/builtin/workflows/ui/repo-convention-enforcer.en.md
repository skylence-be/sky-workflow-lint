---
short_description: Cleans up code style and formatting, then asks you to approve before merging.
long_description: Runs the project's formatting and linting tools, fixes anything it can, and opens the changes for your review. Nothing merges until you approve it.
---
This workflow keeps a repository tidy without anyone having to remember to run the cleanup tools by hand. It checks code formatting, import order, and common lint rules, fixes what it safely can, and lists anything that still needs a person to look at. Before any of it merges, you get a summary of the changes and a chance to approve them or send them back with notes. If you ask for changes, the workflow tries again with your feedback in hand.
