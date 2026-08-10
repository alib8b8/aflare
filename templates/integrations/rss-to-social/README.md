# RSS to Social

Cross-post RSS feed items to Twitter/X and Mastodon automatically.

## Description

Fetch the latest items from an RSS feed, use AI to generate platform-optimized social media posts (Twitter/X, Mastodon, LinkedIn), and publish them automatically. Supports multi-platform cross-posting.

## Install

```bash
aflare install rss-to-social
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export RSS_FEED_URL="https://your-blog.com/feed.xml"
export TWITTER_API_KEY="your-twitter-api-key"
export MASTODON_TOKEN="your-mastodon-access-token"
export MASTODON_INSTANCE="mastodon.social"
```

## Usage

```bash
aflare run templates/rss-to-social/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rss_feed_url` | URL of the RSS feed | Required |
| `twitter_api_key` | Twitter/X API bearer token | Required |
| `mastodon_token` | Mastodon access token | Required |
| `mastodon_instance` | Mastodon instance domain | Required |

## Nodes Used

- `http_request` — Fetch RSS feed, post to Twitter, post to Mastodon
- `xml_parse` — Parse RSS XML feed
- `agent` — AI-powered post generation for each platform
- `file_write` — Save generated posts
- `notify` — Display confirmation

## Output

- `rss-social-posts.md` — All generated posts for reference
- Posts published to Twitter/X and Mastodon

## Schedule

```bash
# Every 4 hours for fresh content
0 */4 * * * aflare run /path/to/templates/rss-to-social/workflow.yaml
```

## Category

integrations