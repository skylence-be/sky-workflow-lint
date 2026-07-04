---
short_description: Implements a labeled GitHub issue and opens a pull request for you to review.
long_description: "Triggered when an issue gets the \"ready-for-sky\" label: writes the implementation, asks you to approve it, then opens a pull request. Reject with notes to have it revised."
---
This workflow turns a labeled GitHub issue into working code. Once an issue is marked ready, it reads the description, implements the change, and runs the project's tests. Before anything is opened for review, you get a chance to approve the implementation or send it back with feedback — the workflow will revise its work and try again. Once approved, it opens a draft pull request linked back to the original issue.
