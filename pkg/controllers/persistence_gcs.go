package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type gcsPersistence struct {
	bucket        string
	storageClient *storage.Client
}

func (p *gcsPersistence) backup(ctx context.Context, state *corev1.Secret) error {
	// Re-use client or create if not yet created
	if p.storageClient == nil {
		var err error
		p.storageClient, err = storage.NewClient(ctx)
		if err != nil {
			return err
		}
	}

	bh := p.storageClient.Bucket(p.bucket)
	_, err := bh.Attrs(ctx)
	if err != nil {
		return err
	}

	path := gcsPath(state.Namespace, state.Name)
	oh := bh.Object(path)

	// Marshal state file first to json then to yaml
	y, err := yaml.Marshal(state)
	if err != nil {
		return err
	}

	// Copy state file to GCS
	owriter := oh.NewWriter(ctx)
	_, err = io.Copy(owriter, bytes.NewBuffer(y))
	if err != nil {
		return err
	}

	if err := owriter.Close(); err != nil {
		return err
	}

	return nil
}

func (p *gcsPersistence) restore(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	var state corev1.Secret

	// Re-use client or create if not yet created
	if p.storageClient == nil {
		var err error
		p.storageClient, err = storage.NewClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	bh := p.storageClient.Bucket(p.bucket)
	_, err := bh.Attrs(ctx)
	if err != nil {
		return nil, err
	}

	// Try to retrieve existing backup
	path := gcsPath(namespace, name)
	oh := bh.Object(path)
	_, err = oh.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		// Return a nil obj indicating there was nothing to restore
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	oreader, err := oh.NewReader(ctx)
	if err != nil {
		return nil, err
	}

	// Copy state file from GCS
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, oreader)
	if err != nil {
		return nil, err
	}

	// Unmarshal state file into secret obj
	if err := yaml.Unmarshal(buf.Bytes(), &state); err != nil {
		return nil, err
	}

	if err := oreader.Close(); err != nil {
		return nil, err
	}

	return nil, nil
}

func gcsPath(namespace, name string) string {
	return fmt.Sprintf("%s/%s.yaml", namespace, name)
}

// Handle errors from the Google Cloud storage client
func handleStorageError(err error) error {
	if err == storage.ErrBucketNotExist {
		return fatal(err)
	}

	if gerr, ok := err.(*googleapi.Error); ok {
		if gerr.Code >= 400 && gerr.Code < 500 {
			// HTTP 40x errors are deemed unrecoverable
			r.recorder.Eventf(ws, "Warning", reason, gerr.Message)
			return workspaceFailure(fmt.Sprintf("%s: %s", reason, gerr.Message)), nil
		}
	}
	r.recorder.Eventf(ws, "Warning", reason, err.Error())
	return nil, err
}

type fatal struct {
	error
}
