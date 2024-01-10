package lfs

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"github.com/openmerlin/gitea_data/modules/setting"

	git_model "github.com/openmerlin/gitea_data/models/git"
	lfs_module "github.com/openmerlin/gitea_data/modules/lfs"
	"github.com/openmerlin/gitea_data/modules/storage"
	"github.com/openmerlin/gitea_data/modules/structs"
)

func BatchHandlerAdapter(ctx *context.Context) {
	var br lfs_module.BatchRequest
	if err := decodeJSON(ctx.Req, &br); err != nil {
		log.Trace("Unable to decode BATCH request vars: Error: %v", err)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}
	if isMultipartTransfers(br.Transfers) {
		log.Trace("handle batch request with multipart transfer")
		MultipartBatchHandler(ctx, &br)
	} else {
		log.Trace("handle batch request with basic transfer")
		BatchHandler(ctx, &br)
	}
}

// MultipartVerifyLink builds a URL for verifying the object in the case multipart.
func (rc *requestContext) MultipartVerifyLink(p lfs_module.Pointer) string {
	return setting.AppURL + path.Join(url.PathEscape(rc.User), url.PathEscape(rc.Repo+".git"), fmt.Sprintf("info/lfs/multipart-verify?oid=%s&size=%s", url.PathEscape(p.Oid), strconv.FormatInt(p.Size, 10)))
}

// MultipartBatchHandler provides the batch api which support multipart
func MultipartBatchHandler(ctx *context.Context, br *lfs_module.BatchRequest) {
	var isUpload bool
	if br.Operation == "upload" {
		isUpload = true
	} else if br.Operation == "download" {
		isUpload = false
	} else {
		log.Trace("Attempt to BATCH with invalid operation: %s", br.Operation)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}

	rc := getRequestContext(ctx)
	repository := getAuthenticatedRepository(ctx, rc, isUpload)
	if repository == nil {
		log.Trace("Unable to get auth repository")
		writeStatus(ctx, http.StatusBadRequest)
		return
	}
	contentStore := lfs_module.NewContentStore()

	var responseObjects []*lfs_module.ObjectResponseWithMultipart

	for _, p := range br.Objects {
		if !p.IsValid() {
			responseObjects = append(responseObjects, buildMultiPartObjectResponse(rc, p, false, false, &lfs_module.ObjectError{
				Code:    http.StatusUnprocessableEntity,
				Message: "Oid or size are invalid",
			}, nil, nil))
			continue
		}

		exists, err := contentStore.Exists(p)
		if err != nil {
			log.Error("Unable to check if LFS OID[%s] exist. Error: %v", p.Oid, rc.User, rc.Repo, err)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}

		meta, err := git_model.GetLFSMetaObjectByOid(ctx, repository.ID, p.Oid)
		if err != nil && err != git_model.ErrLFSObjectNotExist {
			log.Error("Unable to get LFS MetaObject [%s] for %s/%s. Error: %v", p.Oid, rc.User, rc.Repo, err)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}

		if meta != nil && p.Size != meta.Size {
			responseObjects = append(responseObjects, buildMultiPartObjectResponse(rc, p, false, false, &lfs_module.ObjectError{
				Code:    http.StatusUnprocessableEntity,
				Message: fmt.Sprintf("Object %s is not %d bytes", p.Oid, p.Size),
			}, nil, nil))
			continue
		}

		var responseObject *lfs_module.ObjectResponseWithMultipart
		if isUpload {
			var err *lfs_module.ObjectError
			if !exists && setting.LFS.MaxFileSize > 0 && p.Size > setting.LFS.MaxFileSize {
				err = &lfs_module.ObjectError{
					Code:    http.StatusUnprocessableEntity,
					Message: fmt.Sprintf("Size must be less than or equal to %d", setting.LFS.MaxFileSize),
				}
			}

			if exists && meta == nil {
				accessible, err := git_model.LFSObjectAccessible(ctx, ctx.Doer, p.Oid)
				if err != nil {
					log.Error("Unable to check if LFS MetaObject [%s] is accessible. Error: %v", p.Oid, err)
					writeStatus(ctx, http.StatusInternalServerError)
					return
				}
				if accessible {
					_, err := git_model.NewLFSMetaObject(ctx, &git_model.LFSMetaObject{Pointer: p, RepositoryID: repository.ID})
					if err != nil {
						log.Error("Unable to create LFS MetaObject [%s] for %s/%s. Error: %v", p.Oid, rc.User, rc.Repo, err)
						writeStatus(ctx, http.StatusInternalServerError)
						return
					}
				} else {
					exists = false
				}
			}
			//get multipart information
			part, _, verify, errorMessage := contentStore.GenerateMultipartParts(p)
			if errorMessage != nil {
				log.Error("Unable to generate multipart information. Error: %v", p.Oid, errorMessage)
				writeStatus(ctx, http.StatusInternalServerError)
				return
			}

			responseObject = buildMultiPartObjectResponse(rc, p, false, !exists, err, part, verify)
		} else {
			var err *lfs_module.ObjectError
			if !exists || meta == nil {
				err = &lfs_module.ObjectError{
					Code:    http.StatusNotFound,
					Message: http.StatusText(http.StatusNotFound),
				}
			}
			responseObject = buildMultiPartObjectResponse(rc, p, true, false, err, nil, nil)
		}
		responseObjects = append(responseObjects, responseObject)
	}

	respobj := &lfs_module.BatchResponseWithMultiPart{Objects: responseObjects, Transfer: "multipart"}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(respobj); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
}

// MultiPartVerifyHandler merge object and verify oid and its size from the content store
func MultiPartVerifyHandler(ctx *context.Context) {
	size, err := strconv.ParseInt(ctx.Req.URL.Query().Get("size"), 10, 64)
	if err != nil {
		log.Warn("unable to parse object size from query parameter")
		writeStatus(ctx, http.StatusUnprocessableEntity)
		return
	}
	parameter, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		log.Warn("unable to parse request body for additional parameter")
		writeStatus(ctx, http.StatusUnprocessableEntity)
		return
	}

	rc := getRequestContext(ctx)
	repository := getAuthenticatedRepository(ctx, rc, true)
	if repository == nil {
		log.Error("lfs[multipart] failed to authenticate repository")
		writeStatus(ctx, http.StatusUnprocessableEntity)
		return
	}

	var p = lfs_module.Pointer{
		Oid:  ctx.Req.URL.Query().Get("oid"),
		Size: size,
	}

	contentStore := lfs_module.NewContentStore()
	//check whether object exists
	exists, err := contentStore.Exists(p)
	if err != nil {
		log.Error("lfs[multipart] unable to check if LFS OID[%s] exist. Error: %v", p.Oid, err)
		writeStatus(ctx, http.StatusInternalServerError)
		return
	}
	if exists {
		accessible, err := git_model.LFSObjectAccessible(ctx, ctx.Doer, p.Oid)
		if err != nil || !accessible {
			log.Error("lfs[multipart] unable to check if LFS MetaObject [%s] is accessible. Error: %v", p.Oid, err)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}
		log.Error("lfs[multipart] LFS Object already exists", p.Oid)
		writeStatus(ctx, http.StatusOK)
		return
	}
	ok, err := contentStore.CommitAndVerify(p, string(parameter))
	if err != nil {
		log.Error("lfs[multipart] failed to commit and verify LFS object %v", err)
	} else {
		_, err = git_model.NewLFSMetaObject(ctx, &git_model.LFSMetaObject{Pointer: p, RepositoryID: repository.ID})
		if err != nil {
			log.Error("lfs[multipart] failed to create git lfs meta object OID[%s] %v", p.Oid, err)
		}
	}

	status := http.StatusOK
	if err != nil {
		log.Error("lfs[multipart] error commit and verify LFS OID[%s]: %v", p.Oid, err)
		status = http.StatusInternalServerError
	} else if !ok {
		status = http.StatusNotFound
	}
	writeStatus(ctx, status)
}

func buildMultiPartObjectResponse(
	rc *requestContext,
	pointer lfs_module.Pointer,
	download, upload bool,
	err *lfs_module.ObjectError,
	parts []*structs.MultipartObjectPart,
	verify *structs.MultipartEndpoint,
) *lfs_module.ObjectResponseWithMultipart {
	rep := &lfs_module.ObjectResponseWithMultipart{Pointer: pointer}
	if err != nil {
		rep.Error = err
	} else {
		rep.Actions = lfs_module.ObjectResponseActionWithMultipart{}

		header := make(map[string]string)

		if len(rc.Authorization) > 0 {
			header["Authorization"] = rc.Authorization
		}

		if download {
			var link *structs.MultipartEndpoint
			if setting.LFS.Storage.MinioConfig.ServeDirect {
				// If we have a signed url (S3, object storage), redirect to this directly.
				u, err := storage.LFS.URL(pointer.RelativePath(), pointer.Oid)
				if u != nil && err == nil {
					// Presigned url does not need the Authorization header
					// https://github.com/go-gitea/gitea/issues/21525
					delete(header, "Authorization")
					link = &structs.MultipartEndpoint{Href: u.String(), Headers: &header}
				}
			}
			if link == nil {
				link = &structs.MultipartEndpoint{Href: rc.DownloadLink(pointer), Headers: &header}
			}
			rep.Actions.Download = link
		}
		if upload {
			//add parts
			rep.Actions.Parts = parts
			if verify.Headers == nil {
				headers := make(map[string]string)
				verify.Headers = &headers
			}
			for key, value := range header {
				(*verify.Headers)[key] = value
			}
			// This is only needed to workaround https://github.com/git-lfs/git-lfs/issues/3662
			(*verify.Headers)["Accept"] = lfs_module.MediaType
			//add verify
			verify.Href = rc.MultipartVerifyLink(pointer)
			verify.Method = http.MethodPost
			rep.Actions.Verify = verify
		}
	}
	return rep
}

func isMultipartTransfers(transfers []string) bool {
	for _, a := range transfers {
		if a == "multipart" {
			return true
		}
	}
	return false
}

func handleLFSAccessToken(ctx *context.Context, accesToken string, target *repo_model.Repository, mode perm.AccessMode) (*user_model.User, error) {
	token, err := auth_model.GetAccessTokenBySHA(ctx, accesToken)
	if err != nil {
		log.Error("unable to get user access token for lfs operation %v", err)
		return nil, err
	}
	u, err := user_model.GetUserByID(ctx, token.UID)
	log.Trace("Basic Authorization: Valid AccessToken for user[%d]", u.ID)
	if err != nil {
		log.Error("unable to get user id by token for lfs operation %v", err)
		return nil, err
	}
	ctx.Data["IsApiToken"] = true
	ctx.Data["ApiTokenScope"] = token.Scope
	return u, nil
}
