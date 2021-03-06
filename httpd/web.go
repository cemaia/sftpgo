package httpd

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/drakkan/sftpgo/common"
	"github.com/drakkan/sftpgo/dataprovider"
	"github.com/drakkan/sftpgo/kms"
	"github.com/drakkan/sftpgo/utils"
	"github.com/drakkan/sftpgo/version"
	"github.com/drakkan/sftpgo/vfs"
)

const (
	templateBase         = "base.html"
	templateUsers        = "users.html"
	templateUser         = "user.html"
	templateAdmins       = "admins.html"
	templateAdmin        = "admin.html"
	templateConnections  = "connections.html"
	templateFolders      = "folders.html"
	templateFolder       = "folder.html"
	templateMessage      = "message.html"
	templateStatus       = "status.html"
	templateLogin        = "login.html"
	templateChangePwd    = "changepwd.html"
	pageUsersTitle       = "Users"
	pageAdminsTitle      = "Admins"
	pageConnectionsTitle = "Connections"
	pageStatusTitle      = "Status"
	pageFoldersTitle     = "Folders"
	pageChangePwdTitle   = "Change password"
	page400Title         = "Bad request"
	page403Title         = "Forbidden"
	page404Title         = "Not found"
	page404Body          = "The page you are looking for does not exist."
	page500Title         = "Internal Server Error"
	page500Body          = "The server is unable to fulfill your request."
	defaultQueryLimit    = 500
	webDateTimeFormat    = "2006-01-02 15:04:05" // YYYY-MM-DD HH:MM:SS
	redactedSecret       = "[**redacted**]"
)

var (
	templates = make(map[string]*template.Template)
)

type basePage struct {
	Title              string
	CurrentURL         string
	UsersURL           string
	UserURL            string
	AdminsURL          string
	AdminURL           string
	QuotaScanURL       string
	ConnectionsURL     string
	FoldersURL         string
	FolderURL          string
	LogoutURL          string
	ChangeAdminPwdURL  string
	FolderQuotaScanURL string
	StatusURL          string
	UsersTitle         string
	AdminsTitle        string
	ConnectionsTitle   string
	FoldersTitle       string
	StatusTitle        string
	Version            string
	LoggedAdmin        *dataprovider.Admin
}

type usersPage struct {
	basePage
	Users []dataprovider.User
}

type adminsPage struct {
	basePage
	Admins []dataprovider.Admin
}

type foldersPage struct {
	basePage
	Folders []vfs.BaseVirtualFolder
}

type connectionsPage struct {
	basePage
	Connections []common.ConnectionStatus
}

type statusPage struct {
	basePage
	Status ServicesStatus
}

type userPage struct {
	basePage
	User                 dataprovider.User
	RootPerms            []string
	Error                string
	ValidPerms           []string
	ValidSSHLoginMethods []string
	ValidProtocols       []string
	RootDirPerms         []string
	RedactedSecret       string
	IsAdd                bool
}

type adminPage struct {
	basePage
	Admin *dataprovider.Admin
	Error string
	IsAdd bool
}

type changePwdPage struct {
	basePage
	Error string
}

type folderPage struct {
	basePage
	Folder vfs.BaseVirtualFolder
	Error  string
}

type messagePage struct {
	basePage
	Error   string
	Success string
}

type loginPage struct {
	CurrentURL string
	Version    string
	Error      string
}

func loadTemplates(templatesPath string) {
	usersPaths := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateUsers),
	}
	userPaths := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateUser),
	}
	adminsPaths := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateAdmins),
	}
	adminPaths := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateAdmin),
	}
	changePwdPaths := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateChangePwd),
	}
	connectionsPaths := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateConnections),
	}
	messagePath := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateMessage),
	}
	foldersPath := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateFolders),
	}
	folderPath := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateFolder),
	}
	statusPath := []string{
		filepath.Join(templatesPath, templateBase),
		filepath.Join(templatesPath, templateStatus),
	}
	loginPath := []string{
		filepath.Join(templatesPath, templateLogin),
	}
	usersTmpl := utils.LoadTemplate(template.ParseFiles(usersPaths...))
	userTmpl := utils.LoadTemplate(template.ParseFiles(userPaths...))
	adminsTmpl := utils.LoadTemplate(template.ParseFiles(adminsPaths...))
	adminTmpl := utils.LoadTemplate(template.ParseFiles(adminPaths...))
	connectionsTmpl := utils.LoadTemplate(template.ParseFiles(connectionsPaths...))
	messageTmpl := utils.LoadTemplate(template.ParseFiles(messagePath...))
	foldersTmpl := utils.LoadTemplate(template.ParseFiles(foldersPath...))
	folderTmpl := utils.LoadTemplate(template.ParseFiles(folderPath...))
	statusTmpl := utils.LoadTemplate(template.ParseFiles(statusPath...))
	loginTmpl := utils.LoadTemplate(template.ParseFiles(loginPath...))
	changePwdTmpl := utils.LoadTemplate(template.ParseFiles(changePwdPaths...))

	templates[templateUsers] = usersTmpl
	templates[templateUser] = userTmpl
	templates[templateAdmins] = adminsTmpl
	templates[templateAdmin] = adminTmpl
	templates[templateConnections] = connectionsTmpl
	templates[templateMessage] = messageTmpl
	templates[templateFolders] = foldersTmpl
	templates[templateFolder] = folderTmpl
	templates[templateStatus] = statusTmpl
	templates[templateLogin] = loginTmpl
	templates[templateChangePwd] = changePwdTmpl
}

func getBasePageData(title, currentURL string, r *http.Request) basePage {
	return basePage{
		Title:              title,
		CurrentURL:         currentURL,
		UsersURL:           webUsersPath,
		UserURL:            webUserPath,
		AdminsURL:          webAdminsPath,
		AdminURL:           webAdminPath,
		FoldersURL:         webFoldersPath,
		FolderURL:          webFolderPath,
		LogoutURL:          webLogoutPath,
		ChangeAdminPwdURL:  webChangeAdminPwdPath,
		QuotaScanURL:       webQuotaScanPath,
		ConnectionsURL:     webConnectionsPath,
		StatusURL:          webStatusPath,
		FolderQuotaScanURL: webScanVFolderPath,
		UsersTitle:         pageUsersTitle,
		AdminsTitle:        pageAdminsTitle,
		ConnectionsTitle:   pageConnectionsTitle,
		FoldersTitle:       pageFoldersTitle,
		StatusTitle:        pageStatusTitle,
		Version:            version.GetAsString(),
		LoggedAdmin:        getAdminFromToken(r),
	}
}

func renderTemplate(w http.ResponseWriter, tmplName string, data interface{}) {
	err := templates[tmplName].ExecuteTemplate(w, tmplName, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderMessagePage(w http.ResponseWriter, r *http.Request, title, body string, statusCode int, err error, message string) {
	var errorString string
	if body != "" {
		errorString = body + " "
	}
	if err != nil {
		errorString += err.Error()
	}
	data := messagePage{
		basePage: getBasePageData(title, "", r),
		Error:    errorString,
		Success:  message,
	}
	w.WriteHeader(statusCode)
	renderTemplate(w, templateMessage, data)
}

func renderInternalServerErrorPage(w http.ResponseWriter, r *http.Request, err error) {
	renderMessagePage(w, r, page500Title, page500Body, http.StatusInternalServerError, err, "")
}

func renderBadRequestPage(w http.ResponseWriter, r *http.Request, err error) {
	renderMessagePage(w, r, page400Title, "", http.StatusBadRequest, err, "")
}

func renderForbiddenPage(w http.ResponseWriter, r *http.Request, body string) {
	renderMessagePage(w, r, page403Title, "", http.StatusForbidden, nil, body)
}

func renderNotFoundPage(w http.ResponseWriter, r *http.Request, err error) {
	renderMessagePage(w, r, page404Title, page404Body, http.StatusNotFound, err, "")
}

func renderChangePwdPage(w http.ResponseWriter, r *http.Request, error string) {
	data := changePwdPage{
		basePage: getBasePageData(pageChangePwdTitle, webChangeAdminPwdPath, r),
		Error:    error,
	}

	renderTemplate(w, templateChangePwd, data)
}

func renderAddUpdateAdminPage(w http.ResponseWriter, r *http.Request, admin *dataprovider.Admin,
	error string, isAdd bool) {
	currentURL := webAdminPath
	if !isAdd {
		currentURL = fmt.Sprintf("%v/%v", webAdminPath, url.PathEscape(admin.Username))
	}
	data := adminPage{
		basePage: getBasePageData("Add a new user", currentURL, r),
		Admin:    admin,
		Error:    error,
		IsAdd:    isAdd,
	}

	renderTemplate(w, templateAdmin, data)
}

func renderAddUserPage(w http.ResponseWriter, r *http.Request, user dataprovider.User, error string) {
	user.SetEmptySecretsIfNil()
	data := userPage{
		basePage:             getBasePageData("Add a new user", webUserPath, r),
		IsAdd:                true,
		Error:                error,
		User:                 user,
		ValidPerms:           dataprovider.ValidPerms,
		ValidSSHLoginMethods: dataprovider.ValidSSHLoginMethods,
		ValidProtocols:       dataprovider.ValidProtocols,
		RootDirPerms:         user.GetPermissionsForPath("/"),
		RedactedSecret:       redactedSecret,
	}
	renderTemplate(w, templateUser, data)
}

func renderUpdateUserPage(w http.ResponseWriter, r *http.Request, user dataprovider.User, error string) {
	user.SetEmptySecretsIfNil()
	data := userPage{
		basePage:             getBasePageData("Update user", fmt.Sprintf("%v/%v", webUserPath, url.PathEscape(user.Username)), r),
		IsAdd:                false,
		Error:                error,
		User:                 user,
		ValidPerms:           dataprovider.ValidPerms,
		ValidSSHLoginMethods: dataprovider.ValidSSHLoginMethods,
		ValidProtocols:       dataprovider.ValidProtocols,
		RootDirPerms:         user.GetPermissionsForPath("/"),
		RedactedSecret:       redactedSecret,
	}
	renderTemplate(w, templateUser, data)
}

func renderAddFolderPage(w http.ResponseWriter, r *http.Request, folder vfs.BaseVirtualFolder, error string) {
	data := folderPage{
		basePage: getBasePageData("Add a new folder", webFolderPath, r),
		Error:    error,
		Folder:   folder,
	}
	renderTemplate(w, templateFolder, data)
}

func getVirtualFoldersFromPostFields(r *http.Request) []vfs.VirtualFolder {
	var virtualFolders []vfs.VirtualFolder
	formValue := r.Form.Get("virtual_folders")
	for _, cleaned := range getSliceFromDelimitedValues(formValue, "\n") {
		if strings.Contains(cleaned, "::") {
			mapping := strings.Split(cleaned, "::")
			if len(mapping) > 1 {
				vfolder := vfs.VirtualFolder{
					BaseVirtualFolder: vfs.BaseVirtualFolder{
						MappedPath: strings.TrimSpace(mapping[1]),
					},
					VirtualPath: strings.TrimSpace(mapping[0]),
					QuotaFiles:  -1,
					QuotaSize:   -1,
				}
				if len(mapping) > 2 {
					quotaFiles, err := strconv.Atoi(strings.TrimSpace(mapping[2]))
					if err == nil {
						vfolder.QuotaFiles = quotaFiles
					}
				}
				if len(mapping) > 3 {
					quotaSize, err := strconv.ParseInt(strings.TrimSpace(mapping[3]), 10, 64)
					if err == nil {
						vfolder.QuotaSize = quotaSize
					}
				}
				virtualFolders = append(virtualFolders, vfolder)
			}
		}
	}
	return virtualFolders
}

func getUserPermissionsFromPostFields(r *http.Request) map[string][]string {
	permissions := make(map[string][]string)
	permissions["/"] = r.Form["permissions"]
	subDirsPermsValue := r.Form.Get("sub_dirs_permissions")
	for _, cleaned := range getSliceFromDelimitedValues(subDirsPermsValue, "\n") {
		if strings.Contains(cleaned, "::") {
			dirPerms := strings.Split(cleaned, "::")
			if len(dirPerms) > 1 {
				dir := dirPerms[0]
				dir = strings.TrimSpace(dir)
				perms := []string{}
				for _, p := range strings.Split(dirPerms[1], ",") {
					cleanedPerm := strings.TrimSpace(p)
					if len(cleanedPerm) > 0 {
						perms = append(perms, cleanedPerm)
					}
				}
				if len(dir) > 0 {
					permissions[dir] = perms
				}
			}
		}
	}
	return permissions
}

func getSliceFromDelimitedValues(values, delimiter string) []string {
	result := []string{}
	for _, v := range strings.Split(values, delimiter) {
		cleaned := strings.TrimSpace(v)
		if len(cleaned) > 0 {
			result = append(result, cleaned)
		}
	}
	return result
}

func getListFromPostFields(value string) map[string][]string {
	result := make(map[string][]string)
	for _, cleaned := range getSliceFromDelimitedValues(value, "\n") {
		if strings.Contains(cleaned, "::") {
			dirExts := strings.Split(cleaned, "::")
			if len(dirExts) > 1 {
				dir := dirExts[0]
				dir = path.Clean(strings.TrimSpace(dir))
				exts := []string{}
				for _, e := range strings.Split(dirExts[1], ",") {
					cleanedExt := strings.TrimSpace(e)
					if cleanedExt != "" {
						exts = append(exts, cleanedExt)
					}
				}
				if dir != "" {
					if _, ok := result[dir]; ok {
						result[dir] = append(result[dir], exts...)
					} else {
						result[dir] = exts
					}
					result[dir] = utils.RemoveDuplicates(result[dir])
				}
			}
		}
	}
	return result
}

func getFilePatternsFromPostField(valueAllowed, valuesDenied string) []dataprovider.PatternsFilter {
	var result []dataprovider.PatternsFilter
	allowedPatterns := getListFromPostFields(valueAllowed)
	deniedPatterns := getListFromPostFields(valuesDenied)

	for dirAllowed, allowPatterns := range allowedPatterns {
		filter := dataprovider.PatternsFilter{
			Path:            dirAllowed,
			AllowedPatterns: allowPatterns,
		}
		for dirDenied, denPatterns := range deniedPatterns {
			if dirAllowed == dirDenied {
				filter.DeniedPatterns = denPatterns
				break
			}
		}
		result = append(result, filter)
	}
	for dirDenied, denPatterns := range deniedPatterns {
		found := false
		for _, res := range result {
			if res.Path == dirDenied {
				found = true
				break
			}
		}
		if !found {
			result = append(result, dataprovider.PatternsFilter{
				Path:           dirDenied,
				DeniedPatterns: denPatterns,
			})
		}
	}
	return result
}

func getFileExtensionsFromPostField(valueAllowed, valuesDenied string) []dataprovider.ExtensionsFilter {
	var result []dataprovider.ExtensionsFilter
	allowedExtensions := getListFromPostFields(valueAllowed)
	deniedExtensions := getListFromPostFields(valuesDenied)

	for dirAllowed, allowedExts := range allowedExtensions {
		filter := dataprovider.ExtensionsFilter{
			Path:              dirAllowed,
			AllowedExtensions: allowedExts,
		}
		for dirDenied, deniedExts := range deniedExtensions {
			if dirAllowed == dirDenied {
				filter.DeniedExtensions = deniedExts
				break
			}
		}
		result = append(result, filter)
	}
	for dirDenied, deniedExts := range deniedExtensions {
		found := false
		for _, res := range result {
			if res.Path == dirDenied {
				found = true
				break
			}
		}
		if !found {
			result = append(result, dataprovider.ExtensionsFilter{
				Path:             dirDenied,
				DeniedExtensions: deniedExts,
			})
		}
	}
	return result
}

func getFiltersFromUserPostFields(r *http.Request) dataprovider.UserFilters {
	var filters dataprovider.UserFilters
	filters.AllowedIP = getSliceFromDelimitedValues(r.Form.Get("allowed_ip"), ",")
	filters.DeniedIP = getSliceFromDelimitedValues(r.Form.Get("denied_ip"), ",")
	filters.DeniedLoginMethods = r.Form["ssh_login_methods"]
	filters.DeniedProtocols = r.Form["denied_protocols"]
	filters.FileExtensions = getFileExtensionsFromPostField(r.Form.Get("allowed_extensions"), r.Form.Get("denied_extensions"))
	filters.FilePatterns = getFilePatternsFromPostField(r.Form.Get("allowed_patterns"), r.Form.Get("denied_patterns"))
	return filters
}

func getSecretFromFormField(r *http.Request, field string) *kms.Secret {
	secret := kms.NewPlainSecret(r.Form.Get(field))
	if strings.TrimSpace(secret.GetPayload()) == redactedSecret {
		secret.SetStatus(kms.SecretStatusRedacted)
	}
	if strings.TrimSpace(secret.GetPayload()) == "" {
		secret.SetStatus("")
	}
	return secret
}

func getS3Config(r *http.Request) (vfs.S3FsConfig, error) {
	var err error
	config := vfs.S3FsConfig{}
	config.Bucket = r.Form.Get("s3_bucket")
	config.Region = r.Form.Get("s3_region")
	config.AccessKey = r.Form.Get("s3_access_key")
	config.AccessSecret = getSecretFromFormField(r, "s3_access_secret")
	config.Endpoint = r.Form.Get("s3_endpoint")
	config.StorageClass = r.Form.Get("s3_storage_class")
	config.KeyPrefix = r.Form.Get("s3_key_prefix")
	config.UploadPartSize, err = strconv.ParseInt(r.Form.Get("s3_upload_part_size"), 10, 64)
	if err != nil {
		return config, err
	}
	config.UploadConcurrency, err = strconv.Atoi(r.Form.Get("s3_upload_concurrency"))
	return config, err
}

func getGCSConfig(r *http.Request) (vfs.GCSFsConfig, error) {
	var err error
	config := vfs.GCSFsConfig{}

	config.Bucket = r.Form.Get("gcs_bucket")
	config.StorageClass = r.Form.Get("gcs_storage_class")
	config.KeyPrefix = r.Form.Get("gcs_key_prefix")
	autoCredentials := r.Form.Get("gcs_auto_credentials")
	if autoCredentials != "" {
		config.AutomaticCredentials = 1
	} else {
		config.AutomaticCredentials = 0
	}
	credentials, _, err := r.FormFile("gcs_credential_file")
	if err == http.ErrMissingFile {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	defer credentials.Close()
	fileBytes, err := ioutil.ReadAll(credentials)
	if err != nil || len(fileBytes) == 0 {
		if len(fileBytes) == 0 {
			err = errors.New("credentials file size must be greater than 0")
		}
		return config, err
	}
	config.Credentials = kms.NewPlainSecret(string(fileBytes))
	config.AutomaticCredentials = 0
	return config, err
}

func getSFTPConfig(r *http.Request) vfs.SFTPFsConfig {
	config := vfs.SFTPFsConfig{}
	config.Endpoint = r.Form.Get("sftp_endpoint")
	config.Username = r.Form.Get("sftp_username")
	config.Password = getSecretFromFormField(r, "sftp_password")
	config.PrivateKey = getSecretFromFormField(r, "sftp_private_key")
	fingerprintsFormValue := r.Form.Get("sftp_fingerprints")
	config.Fingerprints = getSliceFromDelimitedValues(fingerprintsFormValue, "\n")
	config.Prefix = r.Form.Get("sftp_prefix")
	return config
}

func getAzureConfig(r *http.Request) (vfs.AzBlobFsConfig, error) {
	var err error
	config := vfs.AzBlobFsConfig{}
	config.Container = r.Form.Get("az_container")
	config.AccountName = r.Form.Get("az_account_name")
	config.AccountKey = getSecretFromFormField(r, "az_account_key")
	config.SASURL = r.Form.Get("az_sas_url")
	config.Endpoint = r.Form.Get("az_endpoint")
	config.KeyPrefix = r.Form.Get("az_key_prefix")
	config.AccessTier = r.Form.Get("az_access_tier")
	config.UseEmulator = len(r.Form.Get("az_use_emulator")) > 0
	config.UploadPartSize, err = strconv.ParseInt(r.Form.Get("az_upload_part_size"), 10, 64)
	if err != nil {
		return config, err
	}
	config.UploadConcurrency, err = strconv.Atoi(r.Form.Get("az_upload_concurrency"))
	return config, err
}

func getFsConfigFromUserPostFields(r *http.Request) (dataprovider.Filesystem, error) {
	var fs dataprovider.Filesystem
	provider, err := strconv.Atoi(r.Form.Get("fs_provider"))
	if err != nil {
		provider = int(dataprovider.LocalFilesystemProvider)
	}
	fs.Provider = dataprovider.FilesystemProvider(provider)
	switch fs.Provider {
	case dataprovider.S3FilesystemProvider:
		config, err := getS3Config(r)
		if err != nil {
			return fs, err
		}
		fs.S3Config = config
	case dataprovider.AzureBlobFilesystemProvider:
		config, err := getAzureConfig(r)
		if err != nil {
			return fs, err
		}
		fs.AzBlobConfig = config
	case dataprovider.GCSFilesystemProvider:
		config, err := getGCSConfig(r)
		if err != nil {
			return fs, err
		}
		fs.GCSConfig = config
	case dataprovider.CryptedFilesystemProvider:
		fs.CryptConfig.Passphrase = getSecretFromFormField(r, "crypt_passphrase")
	case dataprovider.SFTPFilesystemProvider:
		fs.SFTPConfig = getSFTPConfig(r)
	}
	return fs, nil
}

func getAdminFromPostFields(r *http.Request) (dataprovider.Admin, error) {
	var admin dataprovider.Admin
	err := r.ParseForm()
	if err != nil {
		return admin, err
	}
	status, err := strconv.Atoi(r.Form.Get("status"))
	if err != nil {
		return admin, err
	}
	admin.Username = r.Form.Get("username")
	admin.Password = r.Form.Get("password")
	admin.Permissions = r.Form["permissions"]
	admin.Email = r.Form.Get("email")
	admin.Status = status
	admin.Filters.AllowList = getSliceFromDelimitedValues(r.Form.Get("allowed_ip"), ",")
	admin.AdditionalInfo = r.Form.Get("additional_info")
	return admin, nil
}

func getUserFromPostFields(r *http.Request) (dataprovider.User, error) {
	var user dataprovider.User
	err := r.ParseMultipartForm(maxRequestSize)
	if err != nil {
		return user, err
	}
	publicKeysFormValue := r.Form.Get("public_keys")
	publicKeys := getSliceFromDelimitedValues(publicKeysFormValue, "\n")
	uid, err := strconv.Atoi(r.Form.Get("uid"))
	if err != nil {
		return user, err
	}
	gid, err := strconv.Atoi(r.Form.Get("gid"))
	if err != nil {
		return user, err
	}
	maxSessions, err := strconv.Atoi(r.Form.Get("max_sessions"))
	if err != nil {
		return user, err
	}
	quotaSize, err := strconv.ParseInt(r.Form.Get("quota_size"), 10, 64)
	if err != nil {
		return user, err
	}
	quotaFiles, err := strconv.Atoi(r.Form.Get("quota_files"))
	if err != nil {
		return user, err
	}
	bandwidthUL, err := strconv.ParseInt(r.Form.Get("upload_bandwidth"), 10, 64)
	if err != nil {
		return user, err
	}
	bandwidthDL, err := strconv.ParseInt(r.Form.Get("download_bandwidth"), 10, 64)
	if err != nil {
		return user, err
	}
	status, err := strconv.Atoi(r.Form.Get("status"))
	if err != nil {
		return user, err
	}
	expirationDateMillis := int64(0)
	expirationDateString := r.Form.Get("expiration_date")
	if len(strings.TrimSpace(expirationDateString)) > 0 {
		expirationDate, err := time.Parse(webDateTimeFormat, expirationDateString)
		if err != nil {
			return user, err
		}
		expirationDateMillis = utils.GetTimeAsMsSinceEpoch(expirationDate)
	}
	fsConfig, err := getFsConfigFromUserPostFields(r)
	if err != nil {
		return user, err
	}
	user = dataprovider.User{
		Username:          r.Form.Get("username"),
		Password:          r.Form.Get("password"),
		PublicKeys:        publicKeys,
		HomeDir:           r.Form.Get("home_dir"),
		VirtualFolders:    getVirtualFoldersFromPostFields(r),
		UID:               uid,
		GID:               gid,
		Permissions:       getUserPermissionsFromPostFields(r),
		MaxSessions:       maxSessions,
		QuotaSize:         quotaSize,
		QuotaFiles:        quotaFiles,
		UploadBandwidth:   bandwidthUL,
		DownloadBandwidth: bandwidthDL,
		Status:            status,
		ExpirationDate:    expirationDateMillis,
		Filters:           getFiltersFromUserPostFields(r),
		FsConfig:          fsConfig,
		AdditionalInfo:    r.Form.Get("additional_info"),
	}
	maxFileSize, err := strconv.ParseInt(r.Form.Get("max_upload_file_size"), 10, 64)
	user.Filters.MaxUploadFileSize = maxFileSize
	return user, err
}

func renderLoginPage(w http.ResponseWriter, error string) {
	data := loginPage{
		CurrentURL: webLoginPath,
		Version:    version.Get().Version,
		Error:      error,
	}
	renderTemplate(w, templateLogin, data)
}

func handleWebAdminChangePwd(w http.ResponseWriter, r *http.Request) {
	renderChangePwdPage(w, r, "")
}

func handleWebAdminChangePwdPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	err := r.ParseForm()
	if err != nil {
		renderChangePwdPage(w, r, err.Error())
		return
	}
	err = doChangeAdminPassword(r, r.Form.Get("current_password"), r.Form.Get("new_password1"),
		r.Form.Get("new_password2"))
	if err != nil {
		renderChangePwdPage(w, r, err.Error())
		return
	}
	handleWebLogout(w, r)
}

func handleWebLogout(w http.ResponseWriter, r *http.Request) {
	c := jwtTokenClaims{}
	c.removeCookie(w)

	http.Redirect(w, r, webLoginPath, http.StatusFound)
}

func handleWebLogin(w http.ResponseWriter, r *http.Request) {
	renderLoginPage(w, "")
}

func handleGetWebAdmins(w http.ResponseWriter, r *http.Request) {
	limit := defaultQueryLimit
	if _, ok := r.URL.Query()["qlimit"]; ok {
		var err error
		limit, err = strconv.Atoi(r.URL.Query().Get("qlimit"))
		if err != nil {
			limit = defaultQueryLimit
		}
	}
	admins := make([]dataprovider.Admin, 0, limit)
	for {
		a, err := dataprovider.GetAdmins(limit, len(admins), dataprovider.OrderASC)
		if err != nil {
			renderInternalServerErrorPage(w, r, err)
			return
		}
		admins = append(admins, a...)
		if len(a) < limit {
			break
		}
	}
	data := adminsPage{
		basePage: getBasePageData(pageAdminsTitle, webAdminsPath, r),
		Admins:   admins,
	}
	renderTemplate(w, templateAdmins, data)
}

func handleWebAddAdminGet(w http.ResponseWriter, r *http.Request) {
	admin := &dataprovider.Admin{Status: 1}
	renderAddUpdateAdminPage(w, r, admin, "", true)
}

func handleWebUpdateAdminGet(w http.ResponseWriter, r *http.Request) {
	username := getURLParam(r, "username")
	admin, err := dataprovider.AdminExists(username)
	if err == nil {
		renderAddUpdateAdminPage(w, r, &admin, "", false)
	} else if _, ok := err.(*dataprovider.RecordNotFoundError); ok {
		renderNotFoundPage(w, r, err)
	} else {
		renderInternalServerErrorPage(w, r, err)
	}
}

func handleWebAddAdminPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	admin, err := getAdminFromPostFields(r)
	if err != nil {
		renderAddUpdateAdminPage(w, r, &admin, err.Error(), true)
		return
	}
	err = dataprovider.AddAdmin(&admin)
	if err != nil {
		renderAddUpdateAdminPage(w, r, &admin, err.Error(), true)
		return
	}
	http.Redirect(w, r, webAdminsPath, http.StatusSeeOther)
}

func handleWebUpdateAdminPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)

	username := getURLParam(r, "username")
	admin, err := dataprovider.AdminExists(username)
	if _, ok := err.(*dataprovider.RecordNotFoundError); ok {
		renderNotFoundPage(w, r, err)
		return
	} else if err != nil {
		renderInternalServerErrorPage(w, r, err)
		return
	}

	updatedAdmin, err := getAdminFromPostFields(r)
	if err != nil {
		renderAddUpdateAdminPage(w, r, &updatedAdmin, err.Error(), false)
		return
	}
	updatedAdmin.ID = admin.ID
	updatedAdmin.Username = admin.Username
	if updatedAdmin.Password == "" {
		updatedAdmin.Password = admin.Password
	}
	claims, err := getTokenClaims(r)
	if err != nil || claims.Username == "" {
		renderAddUpdateAdminPage(w, r, &updatedAdmin, fmt.Sprintf("Invalid token claims: %v", err), false)
		return
	}
	if username == claims.Username {
		if claims.isCriticalPermRemoved(updatedAdmin.Permissions) {
			renderAddUpdateAdminPage(w, r, &updatedAdmin, "You cannot remove these permissions to yourself", false)
			return
		}
		if updatedAdmin.Status == 0 {
			renderAddUpdateAdminPage(w, r, &updatedAdmin, "You cannot disable yourself", false)
			return
		}
	}
	err = dataprovider.UpdateAdmin(&updatedAdmin)
	if err != nil {
		renderAddUpdateAdminPage(w, r, &admin, err.Error(), false)
		return
	}
	http.Redirect(w, r, webAdminsPath, http.StatusSeeOther)
}

func handleGetWebUsers(w http.ResponseWriter, r *http.Request) {
	limit := defaultQueryLimit
	if _, ok := r.URL.Query()["qlimit"]; ok {
		var err error
		limit, err = strconv.Atoi(r.URL.Query().Get("qlimit"))
		if err != nil {
			limit = defaultQueryLimit
		}
	}
	users := make([]dataprovider.User, 0, limit)
	for {
		u, err := dataprovider.GetUsers(limit, len(users), dataprovider.OrderASC)
		if err != nil {
			renderInternalServerErrorPage(w, r, err)
			return
		}
		users = append(users, u...)
		if len(u) < limit {
			break
		}
	}
	data := usersPage{
		basePage: getBasePageData(pageUsersTitle, webUsersPath, r),
		Users:    users,
	}
	renderTemplate(w, templateUsers, data)
}

func handleWebAddUserGet(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("cloneFrom") != "" {
		username := r.URL.Query().Get("cloneFrom")
		user, err := dataprovider.UserExists(username)
		if err == nil {
			user.ID = 0
			user.Username = ""
			if err := user.DecryptSecrets(); err != nil {
				renderInternalServerErrorPage(w, r, err)
				return
			}
			renderAddUserPage(w, r, user, "")
		} else if _, ok := err.(*dataprovider.RecordNotFoundError); ok {
			renderNotFoundPage(w, r, err)
		} else {
			renderInternalServerErrorPage(w, r, err)
		}
	} else {
		user := dataprovider.User{Status: 1}
		renderAddUserPage(w, r, user, "")
	}
}

func handleWebUpdateUserGet(w http.ResponseWriter, r *http.Request) {
	username := getURLParam(r, "username")
	user, err := dataprovider.UserExists(username)
	if err == nil {
		renderUpdateUserPage(w, r, user, "")
	} else if _, ok := err.(*dataprovider.RecordNotFoundError); ok {
		renderNotFoundPage(w, r, err)
	} else {
		renderInternalServerErrorPage(w, r, err)
	}
}

func handleWebAddUserPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	user, err := getUserFromPostFields(r)
	if err != nil {
		renderAddUserPage(w, r, user, err.Error())
		return
	}
	err = dataprovider.AddUser(&user)
	if err == nil {
		http.Redirect(w, r, webUsersPath, http.StatusSeeOther)
	} else {
		renderAddUserPage(w, r, user, err.Error())
	}
}

func handleWebUpdateUserPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	username := getURLParam(r, "username")
	user, err := dataprovider.UserExists(username)
	if _, ok := err.(*dataprovider.RecordNotFoundError); ok {
		renderNotFoundPage(w, r, err)
		return
	} else if err != nil {
		renderInternalServerErrorPage(w, r, err)
		return
	}
	updatedUser, err := getUserFromPostFields(r)
	if err != nil {
		renderUpdateUserPage(w, r, user, err.Error())
		return
	}
	updatedUser.ID = user.ID
	updatedUser.Username = user.Username
	updatedUser.SetEmptySecretsIfNil()
	if updatedUser.Password == "" {
		updatedUser.Password = user.Password
	}
	updateEncryptedSecrets(&updatedUser, user.FsConfig.S3Config.AccessSecret, user.FsConfig.AzBlobConfig.AccountKey,
		user.FsConfig.GCSConfig.Credentials, user.FsConfig.CryptConfig.Passphrase, user.FsConfig.SFTPConfig.Password,
		user.FsConfig.SFTPConfig.PrivateKey)

	err = dataprovider.UpdateUser(&updatedUser)
	if err == nil {
		if len(r.Form.Get("disconnect")) > 0 {
			disconnectUser(user.Username)
		}
		http.Redirect(w, r, webUsersPath, http.StatusSeeOther)
	} else {
		renderUpdateUserPage(w, r, user, err.Error())
	}
}

func handleWebGetStatus(w http.ResponseWriter, r *http.Request) {
	data := statusPage{
		basePage: getBasePageData(pageStatusTitle, webStatusPath, r),
		Status:   getServicesStatus(),
	}
	renderTemplate(w, templateStatus, data)
}

func handleWebGetConnections(w http.ResponseWriter, r *http.Request) {
	connectionStats := common.Connections.GetStats()
	data := connectionsPage{
		basePage:    getBasePageData(pageConnectionsTitle, webConnectionsPath, r),
		Connections: connectionStats,
	}
	renderTemplate(w, templateConnections, data)
}

func handleWebAddFolderGet(w http.ResponseWriter, r *http.Request) {
	renderAddFolderPage(w, r, vfs.BaseVirtualFolder{}, "")
}

func handleWebAddFolderPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	folder := vfs.BaseVirtualFolder{}
	err := r.ParseForm()
	if err != nil {
		renderAddFolderPage(w, r, folder, err.Error())
		return
	}
	folder.MappedPath = r.Form.Get("mapped_path")

	err = dataprovider.AddFolder(&folder)
	if err == nil {
		http.Redirect(w, r, webFoldersPath, http.StatusSeeOther)
	} else {
		renderAddFolderPage(w, r, folder, err.Error())
	}
}

func handleWebGetFolders(w http.ResponseWriter, r *http.Request) {
	limit := defaultQueryLimit
	if _, ok := r.URL.Query()["qlimit"]; ok {
		var err error
		limit, err = strconv.Atoi(r.URL.Query().Get("qlimit"))
		if err != nil {
			limit = defaultQueryLimit
		}
	}
	folders := make([]vfs.BaseVirtualFolder, 0, limit)
	for {
		f, err := dataprovider.GetFolders(limit, len(folders), dataprovider.OrderASC, "")
		if err != nil {
			renderInternalServerErrorPage(w, r, err)
			return
		}
		folders = append(folders, f...)
		if len(f) < limit {
			break
		}
	}

	data := foldersPage{
		basePage: getBasePageData(pageFoldersTitle, webFoldersPath, r),
		Folders:  folders,
	}
	renderTemplate(w, templateFolders, data)
}
