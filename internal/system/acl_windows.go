//go:build windows

package system

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows"
)

// fileAllAccess es el control total sobre un fichero o carpeta, sin bits genéricos.
const fileAllAccess windows.ACCESS_MASK = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1FF

// ProtectTree deja root, y todo lo que contiene, accesible solo para SYSTEM y los administradores.
// Un usuario normal no puede leer la contraseña cifrada.
func ProtectTree(root string) error {
	acl, err := privateACL()
	if err != nil {
		return err
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		return applyACL(path, acl)
	})
	if err != nil {
		return fmt.Errorf("no se pudieron restringir los permisos de %s: %w", root, err)
	}
	return nil
}

func privateACL() (*windows.ACL, error) {
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return nil, err
	}
	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return nil, err
	}
	var pinner runtime.Pinner
	defer pinner.Unpin()
	entries := []windows.EXPLICIT_ACCESS{
		accessEntry(systemSID, windows.TRUSTEE_IS_USER, &pinner),
		accessEntry(adminSID, windows.TRUSTEE_IS_WELL_KNOWN_GROUP, &pinner),
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return nil, fmt.Errorf("no se pudo preparar la lista de permisos: %w", err)
	}
	return acl, nil
}

func accessEntry(sid *windows.SID, kind windows.TRUSTEE_TYPE, pinner *runtime.Pinner) windows.EXPLICIT_ACCESS {
	pinner.Pin(sid)
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: fileAllAccess,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  kind,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

func applyACL(path string, acl *windows.ACL) error {
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	)
}

// PrivateACL reports whether path is protected and grants access to exactly two trustees.
func PrivateACL(path string) (bool, error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	control, _, err := sd.Control()
	if err != nil {
		return false, err
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return false, nil
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return false, err
	}
	return dacl.AceCount == 2, nil
}
