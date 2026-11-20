# Contributing

The useful contributions, in order:

1. A professional (doctor, nurse, paramedic, pharmacist, lawyer, financial adviser, security engineer) reading one chapter and opening an issue for anything wrong or missing.
2. Fixing a quote that `cite check` reports as missing. The weekly CI run finds these.
3. New entries for chapters 4 to 6, which are thin.
4. Sources from outside the US and UK.

## Adding an entry

Entries live in `book/NN-chapter.md`. The format is strict, and `go run ./cmd/cite lint` explains what it doesn't accept.

```markdown
### 7. Fit a carbon monoxide alarm near where people sleep
- Cost: a battery or plug-in CO alarm.
- Gain: CO kills more than 400 Americans a year outside of fires.
- Grade: C
- Source: [CDC, Carbon monoxide poisoning basics](https://www.cdc.gov/carbon-monoxide/about/index.html)
  > Each year, more than 400 Americans die from unintentional CO poisoning not linked to fires
- Note: the numbers are for the US only.
```

Rules:

- Every source needs at least one quote, copied exactly from the page. Use `...` to skip words inside a quote.
- Every number in the Gain line must appear in one of the quotes. Lint enforces this.
- A Note may add context, but if it states a new fact, add a Source with a quote under it.
- Grade A: a systematic review or a guideline graded on trials. B: an official body's summary of research, or official statistics. C: official practical guidance.
- Prefer the page a reader can open: a fact sheet over a PDF, a PubMed abstract over a paywalled article. PDFs are not supported yet.
- Write for someone tired and in a hurry. Short sentences, no jargon, no advice that the source doesn't give.

Before opening a pull request:

```sh
go test ./...
go run ./cmd/cite check -cache .cache
```

## Code

The `cite` tool is plain Go with no dependencies outside the standard library. Keep it that way unless there is a strong reason.
