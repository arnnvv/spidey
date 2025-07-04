-- name: CreateURL :one
INSERT INTO urls (url, status)
VALUES ($1, 'pending')
ON CONFLICT (url) DO NOTHING
RETURNING id;

-- name: GetPendingURLs :many
SELECT url FROM urls WHERE status = 'pending' LIMIT $1;

-- name: UpdateURLStatus :exec
UPDATE urls SET status = $2 WHERE url = $1;

-- name: UpdateURLClassification :exec
UPDATE urls SET classification = $2, confidence = $3 WHERE url = $1;

-- name: MarkURLAsCrawled :exec
UPDATE urls
SET status = 'crawled', content = $2, crawled_at = NOW()
WHERE url = $1;

-- name: MarkURLAsFailed :exec
UPDATE urls
SET status = 'failed', error_message = $2, crawled_at = NOW()
WHERE url = $1;

-- name: MarkURLAsSkipped :exec
UPDATE urls SET status = 'skipped', crawled_at = NOW()
WHERE url = $1;
