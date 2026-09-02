package velero

import velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"

// RestoreByCreationTimestamp sorts a list of Restore by creation timestamp, using their names as a tie breaker.
type RestoreByCreationTimestamp []velerov1api.Restore

func (o RestoreByCreationTimestamp) Len() int      { return len(o) }
func (o RestoreByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o RestoreByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}
