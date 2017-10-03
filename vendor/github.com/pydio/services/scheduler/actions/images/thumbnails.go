package images

import (
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/micro/go-micro/metadata"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/views"
	"go.uber.org/zap"
	"golang.org/x/image/colornames"
	"golang.org/x/net/context"
	"image"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	METADATA_THUMBNAILS       = "ImageThumbnails"
	METADATA_IMAGE_DIMENSIONS = "ImageDimensions"

	METADATA_COMPAT_IS_IMAGE                  = "is_image"
	METADATA_COMPAT_IMAGE_WIDTH               = "image_width"
	METADATA_COMPAT_IMAGE_HEIGHT              = "image_height"
	METADATA_COMPAT_IMAGE_READABLE_DIMENSIONS = "readable_dimension"
)

var (
	thumbnailsActionName = "actions.images.thumbnails"
)

type ThumbnailData struct {
	Format string `json:"format"`
	Size   int    `json:"size"`
	Url    string `json:"url"`
}

type ThumbnailsMeta struct {
	Processing bool
	Thumbnails []ThumbnailData `json:"thumbnails"`
}

type ThumbnailExtractor struct {
	thumbSizes []int
	metaClient tree.NodeReceiverClient
	Client     client.Client
}

// Unique identifier
func (t *ThumbnailExtractor) GetName() string {
	return thumbnailsActionName
}

// Pass parameters
func (t *ThumbnailExtractor) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {
	// Todo : get sizes from parameters
	if action.Parameters != nil {
		t.thumbSizes = []int{}
		if params, ok := action.Parameters["ThumbSizes"]; ok {
			for _, s := range strings.Split(params, ",") {
				parsed, _ := strconv.ParseInt(s, 10, 32)
				t.thumbSizes = append(t.thumbSizes, int(parsed))
			}
		}
	} else {
		t.thumbSizes = []int{512}
	}
	t.metaClient = tree.NewNodeReceiverClient(common.SERVICE_META, cl)
	t.Client = cl
	return nil
}

// Run the actual action code
func (t *ThumbnailExtractor) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 || input.Nodes[0].Size == -1 || input.Nodes[0].Etag == "temporary" {
		// Nothing to do
		return input.WithIgnore(), nil
	}

	node := input.Nodes[0]
	err := t.resize(ctx, node, t.thumbSizes...)
	if err != nil {
		return input.WithError(err), err
	}

	output := input
	output.Nodes[0] = node
	output.AppendOutput(&jobs.ActionOutput{
		Success:    true,
		StringBody: "Created thumbnails for image",
	})

	return output, nil
}

func (t *ThumbnailExtractor) resize(ctx context.Context, node *tree.Node, sizes ...int) error {

	// Open the test image.
	if !node.HasSource() {
		log.Logger(ctx).Error("Error while trying to resize node", zap.Any("node", node))
		return errors.InternalServerError(common.SERVICE_JOBS, "Node does not have enough metadata for Resize (missing Source data)")
	}
	reader, rer := node.ReadFile(ctx)
	if rer != nil {
		return rer
	}
	defer reader.Close()

	src, err := imaging.Decode(reader)
	if err != nil {
		return errors.InternalServerError(common.SERVICE_JOBS, "Error during decode :"+err.Error())
	}

	// Extract dimensions
	bounds := src.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y
	// Send update event right now
	node.SetMeta(METADATA_IMAGE_DIMENSIONS, struct {
		Width  int
		Height int
	}{
		Width:  width,
		Height: height,
	})
	node.SetMeta(METADATA_COMPAT_IS_IMAGE, true)
	node.SetMeta(METADATA_THUMBNAILS, &ThumbnailsMeta{Processing: true})
	node.SetMeta(METADATA_COMPAT_IMAGE_HEIGHT, height)
	node.SetMeta(METADATA_COMPAT_IMAGE_WIDTH, width)
	node.SetMeta(METADATA_COMPAT_IMAGE_READABLE_DIMENSIONS, fmt.Sprintf("%dpx X %dpx", width, height))

	_, err = t.metaClient.UpdateNode(ctx, &tree.UpdateNodeRequest{From: node, To: node})

	if err != nil {
		return err
	}

	log.Logger(ctx).Debug("Thumbnails - Extracted dimension and saved in metadata", zap.Any("dimension", bounds))
	meta := &ThumbnailsMeta{}

	for _, size := range sizes {

		updateMeta, err := t.writeSizeFromSrc(ctx, src, node, size)
		if err != nil {
			return err
		}
		if updateMeta {
			meta.Thumbnails = append(meta.Thumbnails, ThumbnailData{
				Format: "jpg",
				Size:   size,
			})
		}
	}

	if (meta != &ThumbnailsMeta{}) {
		node.SetMeta(METADATA_THUMBNAILS, meta)
	} else {
		node.SetMeta(METADATA_THUMBNAILS, nil)
	}

	log.Logger(ctx).Debug("Updating Meta After Thumbs Generation", zap.Any("meta", meta))
	_, err = t.metaClient.UpdateNode(ctx, &tree.UpdateNodeRequest{From: node, To: node})

	return err
}

func (t *ThumbnailExtractor) writeSizeFromSrc(ctx context.Context, img image.Image, node *tree.Node, targetSize int) (bool, error) {

	localTest := false
	localFolder := ""

	var thumbsClient *minio.Core
	var thumbsBucket string
	objectName := fmt.Sprintf("%s-%d.jpg", node.Uuid, targetSize)

	if localFolder = node.GetStringMeta(common.META_NAMESPACE_NODE_TEST_LOCAL_FOLDER); localFolder != "" {
		localTest = true
	}

	if !localTest {

		var e error
		thumbsClient, thumbsBucket, e = views.GetGenericStoreClient(ctx, common.PYDIO_THUMBSTORE_NAMESPACE, t.Client)
		if e != nil {
			log.Logger(ctx).Error("Cannot find client for thumbstore", zap.Error(e))
			return false, e
		}

		if meta, mOk := metadata.FromContext(ctx); mOk {
			thumbsClient.PrepareMetadata(meta)
			defer thumbsClient.ClearMetadata()
		}

		// First Check if thumb already exists with same original etag
		oi, check := thumbsClient.StatObject(thumbsBucket, objectName, minio.NewHeadReqHeaders())
		log.Logger(ctx).Debug("Object Info", zap.Any("object", oi), zap.Error(check))
		if check == nil {
			foundOriginal := oi.Metadata.Get("X-Amz-Meta-Original-Etag")
			if len(foundOriginal) > 0 && foundOriginal == node.Etag {
				// No update necessary
				log.Logger(ctx).Debug("Ignoring Resize: thumb already exists in store", zap.Any("original", oi))
				return false, nil
			}
		}

	}

	log.Logger(ctx).Debug("WriteSizeFromSrc", zap.String("nodeUuid", node.Uuid))
	// Resize the cropped image to width = 256px preserving the aspect ratio.
	dst := imaging.Resize(img, targetSize, 0, imaging.Lanczos)

	var out io.WriteCloser
	if !localTest {

		var reader io.ReadCloser
		reader, out = io.Pipe()
		defer out.Close()

		go func() {
			defer reader.Close()
			requestMeta := map[string][]string{"Content-Type": {"image/jpeg"}, "X-Amz-Meta-Original-Etag": {node.Etag}}
			_, err := thumbsClient.PutObjectWithMetadata(thumbsBucket, objectName, reader, requestMeta, nil)
			if err != nil {
				log.Logger(ctx).Error("Error while calling PutObjectWithMetadata", zap.Error(err))
			} else {
				log.Logger(ctx).Info("Finished putting thumb for size", zap.Int("size ", targetSize))
			}
		}()

	} else {

		var e error
		out, e = os.OpenFile(filepath.Join(localFolder, objectName), os.O_CREATE|os.O_WRONLY, 0755)
		if e != nil {
			return false, e
		}
		defer out.Close()

	}

	ol := imaging.New(dst.Bounds().Dx(), dst.Bounds().Dy(), colornames.Lightgrey)
	ol = imaging.Overlay(ol, dst, image.Pt(0, 0), 1.0)

	err := imaging.Encode(out, ol, imaging.JPEG)

	log.Logger(ctx).Debug("WriteSizeFromSrc: END", zap.String("nodeUuid", node.Uuid))

	return true, err

}
