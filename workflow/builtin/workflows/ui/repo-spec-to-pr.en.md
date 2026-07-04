---
short_description: Turns a labeled spec into a plan, then code, then a pull request — with your approval at each step.
long_description: "Triggered when an issue gets the \"sky-spec\" label: writes a plan, waits for your approval, implements it, waits for your approval again, then opens a pull request."
---
This workflow takes a bigger piece of work — a spec — and carries it all the way to a pull request, with two checkpoints for your input along the way. First it writes a plan describing what will change and why, and waits for your approval. Once you approve the plan, it implements it and runs the tests, then waits for your approval again before opening a pull request. If you reject at either checkpoint, your feedback is folded back in and that step is retried.
