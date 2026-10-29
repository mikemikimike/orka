/*
Copyright (c) 2026.

MIT License - see LICENSE file for details.
*/

package api

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/orka-agents/orka/api/v1alpha1"
)

// LiveViewConfig turns on the dashboard's live view of browser Tasks.
type LiveViewConfig struct {
	// Enabled mirrors --browser-live-view.
	Enabled bool
	// BrowserWorkerImage is the only image whose Tasks stream frames: the
	// image's Chromium opens the DevTools port the worker screencasts.
	BrowserWorkerImage string
}

const (
	// liveViewContentType is the only frame format the worker sends.
	liveViewContentType = "image/jpeg"
	// maxLiveViewFrameBytes bounds one frame; a 1280x800 JPEG at the
	// worker's quality is about a tenth of this.
	maxLiveViewFrameBytes = 1 << 20
	// liveViewFrameTTL is how long a frame is shown after it arrived. The
	// worker re-sends an unchanged frame every 30 seconds, so only a worker
	// that went away lets a frame expire.
	liveViewFrameTTL = 90 * time.Second
	// liveViewSweepInterval is how soon a finished, deleted, or expired
	// Task's frame leaves memory without anyone asking for it.
	liveViewSweepInterval = 15 * time.Second
	// liveViewReadTimeout bounds a whole frame upload, so a Task cannot hold
	// controller connections open with a slow one.
	liveViewReadTimeout = 10 * time.Second
	// maxLiveViewTasks bounds memory: about one frame per running browser Task.
	maxLiveViewTasks = 256
)

type liveViewFrame struct {
	data []byte
	// at is when the frame last arrived, keepalives included; it drives expiry.
	at time.Time
	// changedAt is when the picture last changed, which the dashboard shows.
	changedAt time.Time
	seq       uint64
	namespace string
	name      string
	uid       types.UID
}

// LiveViewFrames holds the latest screencast frame of each running browser
// Task, in memory only. Nothing is written to disk or kept after the Task
// stops running, and a newer frame replaces the previous one.
type LiveViewFrames struct {
	mu     sync.Mutex
	frames map[string]liveViewFrame
	seq    uint64
	now    func() time.Time
}

// NewLiveViewFrames returns an empty frame holder.
func NewLiveViewFrames() *LiveViewFrames {
	return &LiveViewFrames{frames: map[string]liveViewFrame{}, now: time.Now}
}

// liveViewKey includes the Task UID, so a Task recreated under the same name
// never shows an earlier Task's frames.
func liveViewKey(namespace, name string, uid types.UID) string {
	return namespace + "/" + name + "/" + string(uid)
}

func (f *LiveViewFrames) put(namespace, name string, uid types.UID, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	key := liveViewKey(namespace, name, uid)
	if previous, ok := f.frames[key]; ok && bytes.Equal(previous.data, data) {
		// A keepalive of an unchanged page: it renews expiry, but the picture,
		// its time, and its ETag stay the same.
		previous.at = now
		f.frames[key] = previous
		return
	}
	f.seq++
	f.frames[key] = liveViewFrame{
		data: append([]byte(nil), data...), at: now, changedAt: now, seq: f.seq, namespace: namespace, name: name, uid: uid,
	}
	oldestKey, oldest := "", now
	for k, frame := range f.frames {
		if now.Sub(frame.at) > liveViewFrameTTL {
			delete(f.frames, k)
			continue
		}
		if frame.at.Before(oldest) {
			oldestKey, oldest = k, frame.at
		}
	}
	if len(f.frames) > maxLiveViewTasks && oldestKey != "" {
		delete(f.frames, oldestKey)
	}
}

func (f *LiveViewFrames) get(key string) (liveViewFrame, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	frame, ok := f.frames[key]
	if ok && f.now().Sub(frame.at) > liveViewFrameTTL {
		delete(f.frames, key)
		return liveViewFrame{}, false
	}
	return frame, ok
}

func (f *LiveViewFrames) drop(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.frames, key)
}

// liveViewTaskRunning reports whether the Task with this UID is still running.
type liveViewTaskRunning func(ctx context.Context, namespace, name string, uid types.UID) bool

// sweep drops expired frames and the frames of Tasks that stopped running or
// were deleted, so a finished Task's last picture leaves memory even when
// nobody asks for it again.
func (f *LiveViewFrames) sweep(ctx context.Context, running liveViewTaskRunning) {
	type candidate struct {
		key, namespace, name string
		uid                  types.UID
	}
	f.mu.Lock()
	now := f.now()
	candidates := make([]candidate, 0, len(f.frames))
	for key, frame := range f.frames {
		if now.Sub(frame.at) > liveViewFrameTTL {
			delete(f.frames, key)
			continue
		}
		candidates = append(candidates, candidate{key, frame.namespace, frame.name, frame.uid})
	}
	f.mu.Unlock()
	for _, c := range candidates {
		if !running(ctx, c.namespace, c.name, c.uid) {
			f.drop(c.key)
		}
	}
}

// run sweeps until ctx ends.
func (f *LiveViewFrames) run(ctx context.Context, running liveViewTaskRunning) {
	ticker := time.NewTicker(liveViewSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.sweep(ctx, running)
		}
	}
}

// isLiveViewFramePath matches /internal/v1/live/{namespace}/{taskName}, the
// frame upload route, case-insensitively like the router.
func isLiveViewFramePath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && strings.EqualFold(parts[0], "internal") && strings.EqualFold(parts[1], "v1") &&
		strings.EqualFold(parts[2], "live")
}

// liveViewTask reports whether a Task runs the operator's browser image
// through its entrypoint: only then does the worker screencast its Chromium.
// A command replaces the entrypoint, so that Task has no live view.
func liveViewTask(task *corev1alpha1.Task, browserWorkerImage string) bool {
	return task != nil && browserWorkerImage != "" &&
		task.Spec.Type == corev1alpha1.TaskTypeContainer && task.Spec.Image == browserWorkerImage &&
		len(task.Spec.Command) == 0
}

// GetTaskLiveView handles GET /api/v1/tasks/{id}/live: the latest frame of a
// running browser Task, for the dashboard's view-only live view. It needs the
// same permission as the Task's artifacts. 204 means live view is on and no
// frame has arrived yet; 404 means the Task has no live view now.
func (h *Handlers) GetTaskLiveView(c fiber.Ctx) error {
	id := c.Params("id")
	namespace, err := h.resolveNamespace(c, c.Query(toolNamespaceArg, ""))
	if err != nil {
		return err
	}
	task, err := h.taskAccess().loadReadable(c, "getTaskLiveView", namespace, id)
	if err != nil {
		return err
	}
	if h.liveViewFrames == nil {
		return fiber.NewError(fiber.StatusNotFound, "live view is not enabled")
	}
	if !liveViewTask(task, h.liveViewBrowserImage) {
		return fiber.NewError(fiber.StatusNotFound, "this Task has no live view")
	}
	key := liveViewKey(namespace, task.Name, task.UID)
	if task.Status.Phase != corev1alpha1.TaskPhaseRunning {
		h.liveViewFrames.drop(key)
		return fiber.NewError(fiber.StatusNotFound, "the Task is not running")
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	frame, ok := h.liveViewFrames.get(key)
	if !ok {
		return c.SendStatus(fiber.StatusNoContent)
	}
	etag := `"` + strconv.FormatUint(frame.seq, 10) + `"`
	c.Set(fiber.HeaderETag, etag)
	c.Set("X-Orka-Frame-Time", frame.changedAt.UTC().Format(time.RFC3339Nano))
	if string(c.Request().Header.Peek(fiber.HeaderIfNoneMatch)) == etag {
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Set(fiber.HeaderContentType, liveViewContentType)
	return c.Send(frame.data)
}

// SubmitLiveViewFrame handles POST /internal/v1/live/{namespace}/{taskName}:
// the latest screencast frame from a browser Task's worker. Only the Task's
// current worker may post one, and only while the Task is active.
func (h *InternalHandlers) SubmitLiveViewFrame(c fiber.Ctx) error {
	namespace := c.Params("namespace")
	taskName := c.Params("taskName")
	if namespace == "" || taskName == "" {
		return fiber.NewError(fiber.StatusBadRequest, "namespace and taskName are required")
	}
	if h.liveViewFrames == nil {
		return fiber.NewError(fiber.StatusNotFound, "live view is not enabled")
	}
	if length := c.Request().Header.ContentLength(); length > maxLiveViewFrameBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, fmt.Sprintf("a live view frame is limited to %d bytes", maxLiveViewFrameBytes))
	}
	task, err := h.internalCallerAuthorizer().verifyTaskCaller(c, namespace, taskName)
	if err != nil {
		return err
	}
	if !liveViewTask(task, h.liveViewBrowserImage) {
		return fiber.NewError(fiber.StatusForbidden, "this Task has no live view")
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != liveViewContentType {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "a live view frame must be "+liveViewContentType)
	}
	data := c.Body()
	if len(data) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "empty request body")
	}
	if len(data) > maxLiveViewFrameBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, fmt.Sprintf("a live view frame is limited to %d bytes", maxLiveViewFrameBytes))
	}
	h.liveViewFrames.put(namespace, task.Name, task.UID, data)
	return c.SendStatus(fiber.StatusNoContent)
}

// liveViewTaskRunning keeps a frame while its Task, with the same UID, is
// running. A lookup error keeps it; the frame still expires on its own.
func (s *Server) liveViewTaskRunning(ctx context.Context, namespace, name string, uid types.UID) bool {
	task := &corev1alpha1.Task{}
	if err := s.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, task); err != nil {
		return !apierrors.IsNotFound(err)
	}
	return task.UID == uid && task.Status.Phase == corev1alpha1.TaskPhaseRunning
}
