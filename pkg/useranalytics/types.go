package useranalytics

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// TODO: analyticsEvent and analyticEvent both used in this app, rename one to be more clear
type analyticsEvent struct {
	// userID of the namespace/project owner.  TODO: change to action owner
	userID string
	// pod_add, secret_delete, etc.
	event string
	// Pod, ReplicationController, etc.
	objectKind string
	objectName string
	objectUID  string
	// Namespace/Project. Owner of project is analyticEvent owner.
	objectNamespace string
	// instance ID of the controller to help detect dupes
	controllerID string
	clusterName  string
	properties   map[string]string
	annotations  map[string]string
	// timestamp of event occurrence
	timestamp time.Time
	// the name of the dest to send this event to
	destination string
	// unix time when this event was successfully sent to the destination
	sentTime int64
	// any error message that occurs during sending to destination
	errorMessage string
}

func newEventFromRuntime(typer runtime.ObjectTyper, obj runtime.Object, eventType watch.EventType) (*analyticsEvent, error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("Unable to create object meta for %v", obj)
	}

	kinds, _, err := typer.ObjectKinds(obj)
	if err != nil {
		return nil, err
	}
	simpleTypeName := strings.ToLower(kinds[0].Kind)

	eventName := fmt.Sprintf("%s_%s", simpleTypeName, strings.ToLower(string(eventType)))

	analyticEvent := &analyticsEvent{
		objectKind:      simpleTypeName,
		event:           eventName,
		objectName:      m.GetName(),
		objectNamespace: m.GetNamespace(),
		objectUID:       string(m.GetUID()),
		properties:      make(map[string]string),
		annotations:     make(map[string]string),
		timestamp:       time.Now(),
	}
	for key, value := range m.GetAnnotations() {
		analyticEvent.annotations[key] = value
	}

	// TODO: this is deprecated. Replace with meta.Accessor after rebase.
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("Unable to get ObjectMeta for %v", obj)
	}
	// These funcs are in a newer version of Kube. Rebase is currently underway.
	//	_ = meta.GetCreationTimestamp()
	//	_ = meta.GetDeletionTimestamp()

	switch eventType {
	case watch.Added:
		analyticEvent.timestamp = accessor.GetCreationTimestamp().Time
	case watch.Deleted:
		// if DeletionTimestamp is nil for any reason, analyticEvent.Timestamp is still 'now'.
		// future watch restarts won't receive another Deletion event for the same object.
		if accessor.GetDeletionTimestamp() != nil {
			analyticEvent.timestamp = accessor.GetDeletionTimestamp().Time
		}
	default:
		return nil, fmt.Errorf("unknown event type %v", eventType)
	}

	return analyticEvent, nil
}

func newEvent(typer runtime.ObjectTyper, obj interface{}, eventType watch.EventType) (*analyticsEvent, error) {
	if rt, ok := obj.(runtime.Object); ok {
		return newEventFromRuntime(typer, rt, eventType)
	}
	return nil, fmt.Errorf("Object not runtime.Object:  %v", obj)
}

func (ev *analyticsEvent) Hash() string {
	return fmt.Sprintf("%s,%s,%s,%s,%s,%s", ev.userID, ev.event, ev.objectKind, ev.objectName, ev.objectNamespace, ev.destination)
}
