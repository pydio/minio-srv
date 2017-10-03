package views

import (
	"github.com/pydio/services/common/proto/tree"
	"io"
	"github.com/krolaw/zipstream"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"golang.org/x/net/context"
	"os"
	"path/filepath"
	"strings"
	"github.com/micro/go-micro/errors"
	"io/ioutil"
	"archive/zip"
	"archive/tar"
	"compress/gzip"
	"go.uber.org/zap"
)

type ArchiveReader struct{
	Router Handler
}

func (a *ArchiveReader) openArchiveStream(ctx context.Context, archiveNode *tree.Node) (io.ReadCloser, error) {

	var archive io.ReadCloser
	var openErr error
	if localFolder := archiveNode.GetStringMeta(common.META_NAMESPACE_NODE_TEST_LOCAL_FOLDER); localFolder != ""{
		archive, openErr = os.Open(filepath.Join(localFolder, archiveNode.Uuid))
	} else {
		archive, openErr = a.Router.GetObject(ctx, archiveNode, &GetRequestData{StartOffset:0, Length:-1})
		if openErr != nil {
			log.Logger(ctx).Error("Cannot open Archive", zap.Any("node", archiveNode), zap.Error(openErr))
		}
	}
	return archive, openErr

}

func (a *ArchiveReader) ListChildrenZip(ctx context.Context, archiveNode *tree.Node, parentPath string, stat ...bool) ([]*tree.Node, error) {

	results := []*tree.Node{}

	archive, openErr := a.openArchiveStream(ctx, archiveNode)
	if openErr != nil {
		return results, openErr
	}
	defer archive.Close()

	isStat := false
	if len(stat) > 0 && stat[0] {
		isStat = true
	}

	if !isStat && len(parentPath) > 0 {
		parentPath = strings.TrimSuffix(parentPath, "/") + "/"
	}

	folders := map[string]string{}
	reader := zipstream.NewReader(archive)
	for {
		file, err := reader.Next()
		if err == io.EOF {
			break
		}

		innerPath := strings.TrimPrefix(file.Name, "/")
		if !isStat {
			if !strings.HasPrefix(strings.TrimSuffix(innerPath, "/"), parentPath) {
				continue
			}

			testPath := strings.TrimPrefix(strings.TrimSuffix(innerPath, "/"), parentPath)
			if strings.Contains(testPath, "/") {
				// Check if there is an unreported folder
				f := strings.SplitN(testPath, "/", 2)
				baseDir := f[0]
				if _, already := folders[parentPath + baseDir]; !already {
					// There might be an additional folder here
					innerPath = parentPath + baseDir + "/"
				} else {
					continue
				}
			}

			log.Logger(ctx).Debug("Read File: " + innerPath + "--" + testPath + "--" + parentPath)
		} else {
			if strings.TrimSuffix(innerPath, "/") != parentPath {
				// unreported folder entry in path
				if strings.HasPrefix(innerPath, parentPath + "/") {
					innerPath = parentPath + "/"
				} else {
					continue
				}
			}
		}

		nodeType := tree.NodeType_LEAF
		if strings.HasSuffix(innerPath, "/"){
			nodeType = tree.NodeType_COLLECTION
			innerPath = strings.TrimSuffix(innerPath, "/")
			if _, already := folders[innerPath]; already{
				continue
			}
			folders[innerPath] = innerPath
		}

		node := &tree.Node{
			Path:archiveNode.Path + "/" + innerPath,
			Size:int64(file.UncompressedSize64),
			Type:nodeType,
			MTime:file.ModTime().Unix(),
		}
		results = append(results, node)
		if isStat{
			break
		}
	}

	return results, nil
}

func (a *ArchiveReader) StatChildZip(ctx context.Context, archiveNode *tree.Node, innerPath string) (*tree.Node, error) {

	nodes, err := a.ListChildrenZip(ctx, archiveNode, innerPath, true)
	if err != nil || len(nodes) == 0 {
		return nil, errors.NotFound(VIEWS_LIBRARY_NAME, "File " + innerPath + " not found inside archive " + archiveNode.Path, zap.Error(err))
	}
	return nodes[0], nil

}

func (a *ArchiveReader) ReadChildZip(ctx context.Context, archiveNode *tree.Node, innerPath string) (io.ReadCloser, error){

	// We have to download whole archive to read its content
	var archiveName string
	if localFolder := archiveNode.GetStringMeta(common.META_NAMESPACE_NODE_TEST_LOCAL_FOLDER); localFolder != ""{
		archiveName = filepath.Join(localFolder, archiveNode.Uuid)
	} else {
		remoteReader, openErr := a.Router.GetObject(ctx, archiveNode, &GetRequestData{StartOffset:0, Length:-1})
		if openErr != nil {
			return nil, openErr
		}
		defer remoteReader.Close()
		// Create tmp file
		file, e := ioutil.TempFile("", "pydio-archive-")
		if e != nil {
			return nil, e
		}
		defer file.Close()
		_, e2 := io.Copy(file, remoteReader)
		if e2 != nil {
			return nil, e2
		}
		file.Close()
		remoteReader.Close()
		archiveName = file.Name()
	}

	reader, err := zip.OpenReader(archiveName)
	if err != nil {
		return nil, err
	}

	for _, file := range reader.File {
		if file.Name == innerPath || file.Name == "/" + innerPath {
			fileReader, err := file.Open()
			return fileReader, err
		}
	}
	return nil, errors.NotFound(VIEWS_LIBRARY_NAME, "File " + innerPath + " not found inside archive")

}

func (a *ArchiveReader) ExtractAllZip(ctx context.Context, archiveNode *tree.Node, targetNode *tree.Node, logChannels ...chan string) (error){

	// We have to download whole archive to read its content
	var archiveName string
	if localFolder := archiveNode.GetStringMeta(common.META_NAMESPACE_NODE_TEST_LOCAL_FOLDER); localFolder != ""{
		archiveName = filepath.Join(localFolder, archiveNode.Uuid)
	} else {
		remoteReader, openErr := a.Router.GetObject(ctx, archiveNode, &GetRequestData{StartOffset:0, Length:-1})
		if openErr != nil {
			return openErr
		}
		defer remoteReader.Close()
		// Create tmp file
		file, e := ioutil.TempFile("", "pydio-archive-")
		if e != nil {
			return e
		}
		defer file.Close()
		_, e2 := io.Copy(file, remoteReader)
		if e2 != nil {
			return e2
		}
		file.Close()
		remoteReader.Close()
		archiveName = file.Name()
	}

	reader, err := zip.OpenReader(archiveName)
	if err != nil {
		return err
	}

	for _, file := range reader.File {
		path := filepath.Join(targetNode.GetPath(), strings.TrimSuffix(file.Name, "/"))
		if file.FileInfo().IsDir() {
			_,e :=  a.Router.CreateNode(ctx, &tree.CreateNodeRequest{Node: &tree.Node{Path: path, Type:tree.NodeType_COLLECTION}})
			if e != nil {
				return e
			}
			if len(logChannels) > 0 {
				logChannels[0] <- "Creating directory " + strings.TrimSuffix(file.Name, "/")
			}
		} else {
			fileReader, err := file.Open()
			if err != nil {
				return err
			}
			defer fileReader.Close()

			_, err = a.Router.PutObject(ctx, &tree.Node{Path: path}, fileReader, &PutRequestData{Size: -1})
			if err != nil {
				return err
			}
			if len(logChannels) > 0 {
				logChannels[0] <- "Writing new file " + strings.TrimSuffix(file.Name, "/")
			}
		}

	}


	return nil

}

func (a *ArchiveReader) ListChildrenTar(ctx context.Context, gzipFormat bool, archiveNode *tree.Node, parentPath string, stat ...bool) ([]*tree.Node, error) {

	results := []*tree.Node{}

	archive, openErr := a.openArchiveStream(ctx, archiveNode)
	if openErr != nil {
		return results, openErr
	}
	defer archive.Close()

	isStat := false
	if len(stat) > 0 && stat[0] {
		isStat = true
	}

	if !isStat && len(parentPath) > 0 {
		parentPath = strings.TrimSuffix(parentPath, "/") + "/"
	}

	var tarReader *tar.Reader
	if gzipFormat {
		uncompressedStream, err := gzip.NewReader(archive)
		if err != nil {
			return results, err
		}
		tarReader = tar.NewReader(uncompressedStream)
	} else {
		tarReader = tar.NewReader(archive)
	}

	folders := map[string]string{}
	log.Logger(ctx).Debug("TAR:LIST-START: " + parentPath)
	for {
		file, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		innerPath := strings.TrimPrefix(file.Name, "/")
		log.Logger(ctx).Debug("TAR:LIST " + innerPath)
		if !isStat {
			if !strings.HasPrefix(strings.TrimSuffix(innerPath, "/"), parentPath) {
				continue
			}

			testPath := strings.TrimPrefix(strings.TrimSuffix(innerPath, "/"), parentPath)
			if strings.Contains(testPath, "/") {
				// Check if there is an unreported folder
				f := strings.SplitN(testPath, "/", 2)
				baseDir := f[0]
				if _, already := folders[parentPath + baseDir]; !already {
					// There might be an additional folder here
					innerPath = parentPath + baseDir + "/"
					file.Typeflag = tar.TypeDir
					file.Size = 0
				} else {
					continue
				}
			}

			log.Logger(ctx).Debug("Read File: " + innerPath + "--" + testPath + "--" + parentPath)
		} else {
			if strings.TrimSuffix(innerPath, "/") != parentPath {
				// unreported folder entry in path
				if strings.HasPrefix(innerPath, parentPath + "/") {
					innerPath = parentPath + "/"
					file.Typeflag = tar.TypeDir
					file.Size = 0
				} else {
					continue
				}
			}
		}

		nodeType := tree.NodeType_LEAF
		if file.Typeflag == tar.TypeDir {
			nodeType = tree.NodeType_COLLECTION
			innerPath = strings.TrimSuffix(innerPath, "/")
			if _, already := folders[innerPath]; already{
				continue
			}
			folders[innerPath] = innerPath
		} else if file.Typeflag != tar.TypeReg && file.Typeflag != 0 {
			// Unhandled type, must be Dir or Regular file
			continue
		}

		node := &tree.Node{
			Path:archiveNode.Path + "/" + innerPath,
			Size:int64(file.Size),
			Type:nodeType,
			MTime:file.ModTime.Unix(),
		}
		results = append(results, node)
		if isStat{
			break
		}
	}

	return results, nil
}

func (a *ArchiveReader) StatChildTar(ctx context.Context, gzipFormat bool, archiveNode *tree.Node, innerPath string) (*tree.Node, error) {

	nodes, err := a.ListChildrenTar(ctx, gzipFormat, archiveNode, innerPath, true)
	if err != nil || len(nodes) == 0 {
		return nil, errors.NotFound(VIEWS_LIBRARY_NAME, "File " + innerPath + " not found inside archive " + archiveNode.Path, zap.Error(err))
	}
	return nodes[0], nil

}

func (a *ArchiveReader) ReadChildTar(ctx context.Context, gzipFormat bool, writer io.WriteCloser, archiveNode *tree.Node, innerPath string) (int64, error) {

	// We have to download whole archive to read its content
	var inputStream io.ReadCloser
	var openErr error
	if localFolder := archiveNode.GetStringMeta(common.META_NAMESPACE_NODE_TEST_LOCAL_FOLDER); localFolder != ""{
		inputStream, openErr = os.Open(filepath.Join(localFolder, archiveNode.Uuid))
	} else {
		inputStream, openErr = a.openArchiveStream(ctx, archiveNode)
	}
	if openErr != nil {
		return 0, openErr
	}
	defer inputStream.Close()

	var tarReader *tar.Reader
	if gzipFormat {
		uncompressedStream, err := gzip.NewReader(inputStream)
		if err != nil {
			return 0, err
		}
		tarReader = tar.NewReader(uncompressedStream)
	} else {
		tarReader = tar.NewReader(inputStream)
	}

	for {
		file, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if file == nil {
			return 0, err
		}
		if file.Name == innerPath || file.Name == "/" + innerPath {
			log.Logger(ctx).Debug("Should Start copying tar data to writer")
			written, err := io.Copy(writer, tarReader)
			writer.Close()
			log.Logger(ctx).Debug("After write", zap.Int64("written", written), zap.Error(err))
			return written, err
		}
	}
	return 0, errors.NotFound(VIEWS_LIBRARY_NAME, "File " + innerPath + " not found inside archive")

}

func (a *ArchiveReader) ExtractAllTar(ctx context.Context, gzipFormat bool, archiveNode *tree.Node, targetNode *tree.Node, logChannels ...chan string) (error){

	// We have to download whole archive to read its content
	var inputStream io.ReadCloser
	var openErr error
	if localFolder := archiveNode.GetStringMeta(common.META_NAMESPACE_NODE_TEST_LOCAL_FOLDER); localFolder != ""{
		inputStream, openErr = os.Open(filepath.Join(localFolder, archiveNode.Uuid))
	} else {
		inputStream, openErr = a.openArchiveStream(ctx, archiveNode)
	}
	if openErr != nil {
		return openErr
	}
	defer inputStream.Close()

	var tarReader *tar.Reader
	if gzipFormat {
		uncompressedStream, err := gzip.NewReader(inputStream)
		if err != nil {
			return err
		}
		tarReader = tar.NewReader(uncompressedStream)
	} else {
		tarReader = tar.NewReader(inputStream)
	}

	for {
		file, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if file == nil {
			return err
		}
		path := filepath.Join(targetNode.GetPath(), strings.TrimSuffix(file.Name, "/"))
		if file.FileInfo().IsDir() {
			_,e :=  a.Router.CreateNode(ctx, &tree.CreateNodeRequest{Node: &tree.Node{Path: path, Type:tree.NodeType_COLLECTION}})
			if e != nil {
				return e
			}
			if len(logChannels) > 0 {
				logChannels[0] <- "Creating directory " + strings.TrimSuffix(file.Name, "/")
			}
		} else {
			_, err = a.Router.PutObject(ctx, &tree.Node{Path: path}, tarReader, &PutRequestData{Size: -1})
			if err != nil {
				return err
			}
			if len(logChannels) > 0 {
				logChannels[0] <- "Writing file " + strings.TrimSuffix(file.Name, "/")
			}
		}

	}


	return nil

}


