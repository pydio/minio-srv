package views

import (
	"io"
	"archive/tar"
	"strings"
	"archive/zip"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"github.com/pydio/services/common/log"
	"compress/gzip"
	"go.uber.org/zap"
	"path/filepath"
)

type ArchiveWriter struct{
	Router Handler
}

type walkFunction func(node *tree.Node) error

func (w *ArchiveWriter) walkObjectsWithCallback(ctx context.Context, nodePath string, cb walkFunction) error {

	lNodeClient, err := w.Router.ListNodes(ctx, &tree.ListNodesRequest{
		Node: &tree.Node{
			Path: nodePath,
		},
		Recursive: true,
		Limit:     0,
	})
	if err != nil {
		return err
	}
	defer lNodeClient.Close()
	for {
		clientResponse, err := lNodeClient.Recv()
		if clientResponse == nil || err != nil {
			break
		}
		n := clientResponse.Node
		// Ignore folders
		if !n.IsLeaf() || strings.HasSuffix(n.Path, ".__pydio") {
			continue
		}
		e := cb(n)
		if e != nil {
			log.Logger(ctx).Error("Error trying to add file to archive", zap.String("path", n.Path))
			return e
		}
	}

	return nil

}

func (w *ArchiveWriter) commonRoot(nodes []*tree.Node) string {

	// TODO
	// Assume nodes have same parent for now
	if len(nodes) == 1 && !nodes[0].IsLeaf() {
		return nodes[0].Path
	} else {
		return filepath.Dir(nodes[0].Path)
	}

}

func (w *ArchiveWriter) ZipSelection(ctx context.Context, output io.Writer, nodes []*tree.Node) (int64, error) {

	z := zip.NewWriter(output)
	defer z.Close()
	var totalSizeWritten int64

	parentRoot := w.commonRoot(nodes)

	log.Logger(ctx).Debug("ZipSelection", zap.String("parent", parentRoot), zap.Any("selection", nodes))

	for _, node := range nodes {

		w.walkObjectsWithCallback(ctx, node.Path, func(n *tree.Node) error {

			if n.Size <= 0 {
				return nil
			}
			internalPath := strings.TrimPrefix(n.Path, parentRoot)
			log.Logger(ctx).Info("Adding file to archive: ", zap.String("path", internalPath), zap.Any("node", n))
			header := &zip.FileHeader{
				Name:               internalPath,
				Method:             zip.Deflate,
				UncompressedSize64: uint64(n.Size),
			}
			header.SetMode(0777)
			header.SetModTime(n.GetModTime())
			zW, e := z.CreateHeader(header)
			if e != nil {
				log.Logger(ctx).Error("Error while creating path", zap.String("path", internalPath), zap.Error(e))
				return e
			}
			r, e1 := w.Router.GetObject(ctx, n, &GetRequestData{StartOffset: 0, Length: -1})
			if e1 != nil {
				log.Logger(ctx).Error("Error while getting object", zap.String("path", n.Path), zap.Error(e1))
				return e1
			}
			defer r.Close()
			written, e2 := io.Copy(zW, r)
			if e2 != nil {
				log.Logger(ctx).Error("Error while copying streams", zap.Error(e2))
				return e2
			}
			totalSizeWritten += written

			return nil
		})

	}

	log.Logger(ctx).Debug("Total Size Written", zap.Int64("size", totalSizeWritten))

	return totalSizeWritten, nil
}

func (w *ArchiveWriter) TarSelection(ctx context.Context, output io.Writer, gzipFile bool, nodes []*tree.Node) (int64, error) {

	var tw *tar.Writer
	var totalSizeWritten int64

	if gzipFile {
		// set up the gzip writer
		gw := gzip.NewWriter(output)
		defer gw.Close()

		tw = tar.NewWriter(gw)
		defer tw.Close()
	} else {
		tw = tar.NewWriter(output)
		defer tw.Close()
	}

	parentRoot := w.commonRoot(nodes)

	for _, node := range nodes {

		err := w.walkObjectsWithCallback(ctx, node.Path, func(n *tree.Node) error {

			internalPath := strings.TrimPrefix(n.Path, parentRoot)
			if n.Size <= 0 {
				return nil
			}
			header := &tar.Header{
				Name:    internalPath,
				ModTime: n.GetModTime(),
				Size:    n.Size,
				Mode:    0777,
			}
			if ! n.IsLeaf() {
				header.Typeflag = tar.TypeDir
			} else {
				header.Typeflag = tar.TypeReg
			}
			log.Logger(ctx).Info("Adding file to archive: ", zap.String("path", internalPath), zap.Any("node", n))
			e := tw.WriteHeader(header)
			if e != nil {
				log.Logger(ctx).Error("Error while creating path", zap.String("path", internalPath), zap.Error(e))
				return e
			}
			reader, e1 := w.Router.GetObject(ctx, n, &GetRequestData{StartOffset: 0, Length: -1})
			defer reader.Close()
			if e1 != nil {
				log.Logger(ctx).Error("Error while getting object and writing to tarball", zap.String("path", internalPath), zap.Error(e1))
				return e1
			}
			size, _ := io.Copy(tw, reader)
			totalSizeWritten += size

			return nil
		})

		if err != nil {
			return totalSizeWritten, err
		}

	}

	return totalSizeWritten, nil

}
