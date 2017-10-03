package images

import (
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	"github.com/rwcarlsen/goexif/exif"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

const (
	METADATA_EXIF               = "ImageExif"
	METADATA_GEOLOCATION        = "GeoLocation"
	METADATA_COMPAT_ORIENTATION = "image_exif_orientation"
)

var (
	exifTaskName = "actions.images.exif"
)

type ExifProcessor struct {
	metaClient tree.NodeReceiverClient
}

func (e *ExifProcessor) GetName() string {
	return exifTaskName
}

func (e *ExifProcessor) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {

	e.metaClient = tree.NewNodeReceiverClient(common.SERVICE_META, cl)
	return nil
}

func (e *ExifProcessor) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 || input.Nodes[0].Size == -1 || input.Nodes[0].Etag == "temporary" {
		return input.WithIgnore(), nil
	}
	node := input.Nodes[0]
	exifData, err := e.ExtractExif(ctx, node)

	if err != nil {
		log.Logger(ctx).Error("Could not extract exif : ", zap.Error(err), zap.Any("ctx", ctx))
		return input.WithError(err), err
	}

	if exifData == nil {
		log.Logger(ctx).Debug("No Exif extracted")
		return input, nil
	}

	output := input
	node.SetMeta(METADATA_EXIF, exifData)
	orientation, oe := exifData.Get(exif.Orientation)
	if oe == nil {
		t := orientation.String()
		if t != "" {
			node.SetMeta(METADATA_COMPAT_ORIENTATION, t)
		}
	}
	lat, long, err := exifData.LatLong()
	if err == nil {
		geoLocation := map[string]float64{
			"lat": lat,
			"lon": long,
		}
		node.SetMeta(METADATA_GEOLOCATION, geoLocation)
	}

	e.metaClient.UpdateNode(ctx, &tree.UpdateNodeRequest{From: node, To: node})

	output.Nodes[0] = node
	output.AppendOutput(&jobs.ActionOutput{
		Success:    true,
		StringBody: "Successfully Extracted EXIF data",
	})

	return output, nil
}

func (e *ExifProcessor) ExtractExif(ctx context.Context, node *tree.Node) (*exif.Exif, error) {

	// Open the test image.
	if !node.HasSource() {
		return nil, errors.InternalServerError(common.SERVICE_JOBS, "Node does not have enough metadata")
	}
	reader, rer := node.ReadFile(ctx)
	if rer != nil {
		return nil, rer
	}
	defer reader.Close()

	// Optionally register camera makenote data parsing - currently Nikon and
	// Canon are supported.
	// exif.RegisterParsers(mknote.All...)
	x, err := exif.Decode(reader)
	if err != nil {
		return nil, err
	}

	return x, nil
}
