package branding

import "time"

type SchoolBranding struct {
	SchoolID, SystemTitle, SchoolDisplayName                                                    string
	LogoLightStorageKey, LogoDarkStorageKey, FaviconStorageKey, LoginBackgroundStorageKey       *string
	PrimaryColor, SecondaryColor, AccentColor, WelcomeText, CertificateTitle, CertificateFooter string
	CertificateLogoStorageKey                                                                   *string
	UpdatedAt                                                                                   time.Time
	UpdatedBy                                                                                   string
}
