package gui

import (
	"path/filepath"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/x/cosh"
	"github.com/rodrigocfd/windigo/x/winsh"
)

// pickExe shows the system Open dialog for an executable, starting at current when it exists.
func pickExe(owner win.HWND, title, programs, current string) (string, bool) {
	rel := win.NewOleReleaser()
	defer rel.Release()

	var fod *winsh.IFileOpenDialog
	if win.CoCreateInstance(rel, &cosh.CLSID_FileOpenDialog, nil, co.CLSCTX_INPROC_SERVER, &fod) != nil {
		return "", false
	}
	opts, _ := fod.GetOptions()
	fod.SetOptions(opts | cosh.FOS_FORCEFILESYSTEM | cosh.FOS_FILEMUSTEXIST | cosh.FOS_NOCHANGEDIR)
	fod.SetTitle(title)
	fod.SetFileTypes([]winsh.COMDLG_FILTERSPEC{
		{Name: "HWiNFO (HWiNFO*.exe)", Spec: "HWiNFO*.exe"},
		{Name: programs + " (*.exe)", Spec: "*.exe"},
	})
	fod.SetFileTypeIndex(1)
	if current != "" {
		var dir *winsh.IShellItem
		if winsh.SHCreateItemFromParsingName(rel, filepath.Dir(current), &dir) == nil {
			fod.SetFolder(dir)
			fod.SetFileName(filepath.Base(current))
		}
	}

	if ok, _ := fod.Show(owner); !ok {
		return "", false
	}
	item, err := fod.GetResult(rel)
	if err != nil {
		return "", false
	}
	path, err := item.GetDisplayName(cosh.SIGDN_FILESYSPATH)
	return path, err == nil && path != ""
}
