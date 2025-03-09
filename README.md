# plexctl

A no-frills, Go-based CLI for the Perplexity API.

## Installation

```bash
go install github.com/nathanleclaire/plexctl/cmd/plexctl@latest
```

## Configuration

Set your Perplexity API token:

```bash
export PERPLEXITY_API_TOKEN=your-token-here
```

Or pass it directly:

```bash
plexctl --token=your-token-here [command]
```

## Commands

### Get a Completion

Generate a quick response to any query:

```bash
$ plexctl get "Write the most San Francisco sentence ever"
As the fog rolled in off the Pacific, a cable car rattled up Nob Hill, passing
by the historic mansions of the robber barons, while a group of tech
entrepreneurs sipped coffee in a trendy café in the Mission, discussing their
latest startup idea, all just a stone's throw from the vibrant streets of
Chinatown and the iconic Golden Gate Bridge.

Citations:
[0] - sfgate.com
[1] - wikipedia.org
```

### Manage Threads

List your conversation threads:

```bash
$ plexctl thread
THREAD ID  FIRST USER MESSAGE
GpunfKmq   write the most bay area sentence possible
9Pme8Nvc   what's a cool dinosaur i never heard of?
5TGFAeda   tell me a yo momma joke
FxjcGToh   hell world
9PFRvf5U   what are server side events?
```

View a specific thread:

```bash
$ plexctl thread get GpunfKmq
THREAD: GpunfKmq

[0] USER:
write the most bay area sentence possible

[1] ASSISTANT:
Here's a sentence that captures the essence of the Bay Area:

"As I sipped a pour-over coffee at a trendy cafe in Haight-Ashbury, I pondered
the tech startup I was about to pitch in Silicon Valley, while simultaneously
worrying about the housing market and the next Warriors game."
```
