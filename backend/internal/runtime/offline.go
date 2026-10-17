package runtime

import (
	"context"
	"sort"
)

// OfflineArtifactは外部通信なしでLabを開始するために事前配布する対象である。
type OfflineArtifact struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision,omitempty"`
	Digest    string `json:"digest,omitempty"`
	Platform  string `json:"platform,omitempty"`
}

// OfflineArtifactsは保存済みの固定Lockfileから、事前配布対象だけを列挙する。
// 認証情報やプロキシ設定はSessionにもLockfileにも含めない。
func (manager *Manager) OfflineArtifacts(ctx context.Context, id string) ([]OfflineArtifact, error) {
	if !idPattern.MatchString(id) {
		return nil, ErrNotFound
	}
	session, err := manager.store.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := verifyArtifact(session); err != nil {
		return nil, err
	}
	lock, err := decodeLock(session.Lockfile)
	if err != nil {
		return nil, err
	}
	result := []OfflineArtifact{{Kind: "git-source", Reference: lock.Source.URL, Revision: lock.Source.Revision}}
	services := make([]string, 0, len(lock.Images))
	for service := range lock.Images {
		services = append(services, service)
	}
	sort.Strings(services)
	for _, service := range services {
		image := lock.Images[service]
		result = append(result, OfflineArtifact{Kind: "container-image", Reference: image.Reference, Digest: image.Digest, Platform: image.Platform})
	}
	return result, nil
}
