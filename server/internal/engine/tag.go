package engine

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// TagImage applies an additional repository:tag to an existing image.
func (c *Client) TagImage(ctx context.Context, source, repo, tag string) error {
	q := url.Values{}
	q.Set("repo", repo)
	if tag != "" {
		q.Set("tag", tag)
	}
	resp, err := c.Do(ctx, http.MethodPost, "/images/"+url.PathEscape(source)+"/tag?"+q.Encode(), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
