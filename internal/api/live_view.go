/*
Copyright (c) 2026.

MIT License - see LICENSE file for details.
*/

package api

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
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
	// liveViewFrameTTL is how long a frame is shown after it arrived. A page
	// that stops repainting sends nothing, so an older frame is still the
	// current picture; only a worker that went away leaves it stale.
	liveViewFrameTTL = 2 * time.Minute
	// maxLiveViewTasks bounds memory: about one frame per running browser Task.
	maxLiveViewTasks = 256
)

type liveViewFrame struct {
	data []byte
	at   time.Time
	seq  uint64
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

func (f *LiveViewFrames) put(key string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	f.seq++
	f.frames[key] = liveViewFrame{data: append([]byte(nil), data...), at: now, seq: f.seq}
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

// liveViewTask reports whether a Task runs the operator's browser image, the
// only image whose Chromium the worker can screencast.
func liveViewTask(task *corev1alpha1.Task, browserWorkerImage string) bool {
	return task != nil && browserWorkerImage != "" &&
		task.Spec.Type == corev1alpha1.TaskTypeContainer && task.Spec.Image == browserWorkerImage
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
	c.Set("X-Orka-Frame-Time", frame.at.UTC().Format(time.RFC3339Nano))
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
	h.liveViewFrames.put(liveViewKey(namespace, task.Name, task.UID), data)
	return c.SendStatus(fiber.StatusNoContent)
}
