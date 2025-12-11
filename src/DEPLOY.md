# Deployment Guide

This guide explains how to deploy the fraud-agent Cloud Function to Google Cloud Functions (2nd gen).

## Prerequisites

1. **Google Cloud SDK**: Install and configure `gcloud` CLI

   ```bash
   gcloud auth login
   gcloud config set project YOUR_PROJECT_ID
   ```

2. **Required Permissions**: Ensure you have the following IAM roles:

   - Cloud Functions Admin
   - Service Account User
   - Storage Admin (for source code upload)

3. **Required Environment Variable**:
   - `AI_MODELS`/provider API key - see below

## Environment Variables

### Required

- `AI_MODELS` - Comma-separated list of `provider:model` agents to run (e.g. `openai:gpt-5-mini,claude:claude-3-5-sonnet-20241022`). Defaults to `openai:gpt-5-mini` when unset.
- API keys for the providers you list:
  - `OPENAI_API_KEY` for OpenAI
  - `CLAUDE_API_KEY` or `ANTHROPIC_API_KEY` for Claude
  - `GEMINI_API_KEY` for Gemini

### Optional

- `AI_SYSTEM_PROMPT` - Custom system prompt (falls back to `OPENAI_SYSTEM_PROMPT`, then built-in default)
- `OPENAI_MODEL`, `CLAUDE_MODEL`, `GEMINI_MODEL` - Per-provider default models used when omitted in `AI_MODELS`
- `AI_REASONING_MODELS` / `AI_REASONING_MODEL` - Optional second-layer agents (provider:model) invoked only when a primary agent returns `block` (defaults to `openai:o3-mini`)
- `FINYA_API_URL` - Finya.de API endpoint (defaults to "https://api.finya.de/v1/aiDecisionEvent")
- `FINYA_API_KEY` - Finya.de API key for authentication
- `PRESHARED_KEY` - If set, incoming requests must provide this key via `X-Preshared-Key` or `X-Api-Key` header

## Deployment

### Quick Deploy

From the `cloudfunction` directory:

```bash
export AI_MODELS=openai:gpt-5-mini
export OPENAI_API_KEY=your-openai-api-key
./deploy.sh
```

### Custom Configuration

You can customize the deployment by setting environment variables:

```bash
export FUNCTION_NAME=ProcessTickets
export REGION=us-central1
export PROJECT_ID=your-project-id
export RUNTIME=go124  # or go125
export AI_MODELS=openai:gpt-5-mini
export OPENAI_API_KEY=your-openai-api-key
export FINYA_API_URL=https://api.finya.de/v1/aiDecisionEvent
export FINYA_API_KEY=your-finya-api-key

./deploy.sh
```

### Manual Deployment

If you prefer to deploy manually without the script:

```bash
gcloud functions deploy ProcessTickets \
  --gen2 \
  --runtime=go124 \
  --region=us-central1 \
  --source=. \
  --entry-point=ProcessTickets \
  --trigger-http \
  --allow-unauthenticated \
  --set-env-vars="AI_MODELS=openai:gpt-5-mini,OPENAI_API_KEY=your-key" \
  --project=your-project-id
```

## Post-Deployment

### Get Function URL

After deployment, get the function URL:

```bash
gcloud functions describe ProcessTickets \
  --gen2 \
  --region=us-central1 \
  --format='value(serviceConfig.uri)'
```

### Test the Function

Test the deployed function with a POST request:

```bash
curl -X POST https://YOUR-FUNCTION-URL \
  -H "Content-Type: application/json" \
  -d '[{
    "userId": "test-user-123",
    "suspicions": {"risk_score": 0.8},
    "userProfile": {"signup_country": "DE"}
  }]'
```

### View Logs

Monitor function logs:

```bash
gcloud functions logs read ProcessTickets \
  --gen2 \
  --region=us-central1 \
  --limit=50
```

## Updating Environment Variables

To update environment variables after deployment:

```bash
gcloud functions deploy ProcessTickets \
  --gen2 \
  --region=us-central1 \
  --update-env-vars="AI_MODELS=openai:gpt-5-mini,OPENAI_API_KEY=new-key" \
  --project=your-project-id
```

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure you have the required IAM roles
2. **Invalid Runtime**: Make sure you're using `go124` or `go125`
3. **Missing Environment Variables**: The function requires `AI_MODELS` (or uses the default) and the corresponding provider API keys at minimum
4. **Build Failures**: Check that all dependencies are in `go.mod` and `go.sum`

### Verify Deployment

Check function status:

```bash
gcloud functions describe ProcessTickets \
  --gen2 \
  --region=us-central1
```

## Runtime Versions

Supported Go runtimes for Google Cloud Functions (2nd gen):

- `go124` - Go 1.24
- `go125` - Go 1.25

The function is configured to use `go124` by default. To use `go125`, set `RUNTIME=go125` before running the deployment script.
