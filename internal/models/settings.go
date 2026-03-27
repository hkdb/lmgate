package models

import (
	"database/sql"
	"strconv"
)

type GeneralSettings struct {
	RateLimitEnabled         bool   `json:"rate_limit_enabled"`
	RateLimitDefaultRPM      int    `json:"rate_limit_default_rpm"`
	APILogEnabled            bool   `json:"api_log_enabled"`
	APILogRetentionDays      int    `json:"api_log_retention_days"`
	AdminLogEnabled          bool   `json:"admin_log_enabled"`
	AdminLogRetentionDays    int    `json:"admin_log_retention_days"`
	SecurityLogEnabled       bool   `json:"security_log_enabled"`
	SecurityLogRetentionDays int    `json:"security_log_retention_days"`
	AuditFlushInterval       int    `json:"audit_flush_interval"`
	MaxFailedLogins          int    `json:"max_failed_logins"`
	PasswordMinLength        int    `json:"password_min_length"`
	PasswordRequireSpecial   bool   `json:"password_require_special"`
	PasswordRequireNumber    bool   `json:"password_require_number"`
	UserCacheTTL             int    `json:"user_cache_ttl"`
	Enforce2FA               bool   `json:"enforce_2fa"`
	PasswordExpiryDays       int    `json:"password_expiry_days"`
	AdminAllowedNetworks     string `json:"admin_allowed_networks"`
	GatewayAllowedNetworks   string `json:"gateway_allowed_networks"`
}

func GetGeneralSettings(db *sql.DB, defaults GeneralSettings) (GeneralSettings, error) {
	s := defaults

	rows, err := db.Query(`SELECT key, value FROM app_settings WHERE key IN ('rate_limit_enabled', 'rate_limit_default_rpm', 'api_log_enabled', 'api_log_retention_days', 'admin_log_enabled', 'admin_log_retention_days', 'security_log_enabled', 'security_log_retention_days', 'audit_flush_interval', 'max_failed_logins', 'password_min_length', 'password_require_special', 'password_require_number', 'user_cache_ttl', 'enforce_2fa', 'password_expiry_days', 'admin_allowed_networks', 'gateway_allowed_networks')`)
	if err != nil {
		return s, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return s, err
		}
		switch key {
		case "rate_limit_enabled":
			s.RateLimitEnabled = value == "true"
		case "rate_limit_default_rpm":
			if n, err := strconv.Atoi(value); err == nil {
				s.RateLimitDefaultRPM = n
			}
		case "api_log_enabled":
			s.APILogEnabled = value == "true"
		case "api_log_retention_days":
			if n, err := strconv.Atoi(value); err == nil {
				s.APILogRetentionDays = n
			}
		case "admin_log_enabled":
			s.AdminLogEnabled = value == "true"
		case "admin_log_retention_days":
			if n, err := strconv.Atoi(value); err == nil {
				s.AdminLogRetentionDays = n
			}
		case "security_log_enabled":
			s.SecurityLogEnabled = value == "true"
		case "security_log_retention_days":
			if n, err := strconv.Atoi(value); err == nil {
				s.SecurityLogRetentionDays = n
			}
		case "audit_flush_interval":
			if n, err := strconv.Atoi(value); err == nil {
				s.AuditFlushInterval = n
			}
		case "max_failed_logins":
			if n, err := strconv.Atoi(value); err == nil {
				s.MaxFailedLogins = n
			}
		case "password_min_length":
			if n, err := strconv.Atoi(value); err == nil {
				s.PasswordMinLength = n
			}
		case "password_require_special":
			s.PasswordRequireSpecial = value == "true"
		case "password_require_number":
			s.PasswordRequireNumber = value == "true"
		case "user_cache_ttl":
			if n, err := strconv.Atoi(value); err == nil {
				s.UserCacheTTL = n
			}
		case "enforce_2fa":
			s.Enforce2FA = value == "true"
		case "password_expiry_days":
			if n, err := strconv.Atoi(value); err == nil {
				s.PasswordExpiryDays = n
			}
		case "admin_allowed_networks":
			s.AdminAllowedNetworks = value
		case "gateway_allowed_networks":
			s.GatewayAllowedNetworks = value
		}
	}

	return s, rows.Err()
}

func GetAppSetting(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func SetAppSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO app_settings (key, value, updated_at) VALUES (?, ?, datetime('now'))`, key, value)
	return err
}

func SaveGeneralSettings(db *sql.DB, s GeneralSettings) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO app_settings (key, value, updated_at) VALUES (?, ?, datetime('now'))`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if _, err := stmt.Exec("rate_limit_enabled", strconv.FormatBool(s.RateLimitEnabled)); err != nil {
		return err
	}
	if _, err := stmt.Exec("rate_limit_default_rpm", strconv.Itoa(s.RateLimitDefaultRPM)); err != nil {
		return err
	}
	if _, err := stmt.Exec("api_log_enabled", strconv.FormatBool(s.APILogEnabled)); err != nil {
		return err
	}
	if _, err := stmt.Exec("api_log_retention_days", strconv.Itoa(s.APILogRetentionDays)); err != nil {
		return err
	}
	if _, err := stmt.Exec("admin_log_enabled", strconv.FormatBool(s.AdminLogEnabled)); err != nil {
		return err
	}
	if _, err := stmt.Exec("admin_log_retention_days", strconv.Itoa(s.AdminLogRetentionDays)); err != nil {
		return err
	}
	if _, err := stmt.Exec("security_log_enabled", strconv.FormatBool(s.SecurityLogEnabled)); err != nil {
		return err
	}
	if _, err := stmt.Exec("security_log_retention_days", strconv.Itoa(s.SecurityLogRetentionDays)); err != nil {
		return err
	}
	if _, err := stmt.Exec("audit_flush_interval", strconv.Itoa(s.AuditFlushInterval)); err != nil {
		return err
	}
	if _, err := stmt.Exec("max_failed_logins", strconv.Itoa(s.MaxFailedLogins)); err != nil {
		return err
	}
	if _, err := stmt.Exec("password_min_length", strconv.Itoa(s.PasswordMinLength)); err != nil {
		return err
	}
	if _, err := stmt.Exec("password_require_special", strconv.FormatBool(s.PasswordRequireSpecial)); err != nil {
		return err
	}
	if _, err := stmt.Exec("password_require_number", strconv.FormatBool(s.PasswordRequireNumber)); err != nil {
		return err
	}
	if _, err := stmt.Exec("user_cache_ttl", strconv.Itoa(s.UserCacheTTL)); err != nil {
		return err
	}
	if _, err := stmt.Exec("enforce_2fa", strconv.FormatBool(s.Enforce2FA)); err != nil {
		return err
	}
	if _, err := stmt.Exec("password_expiry_days", strconv.Itoa(s.PasswordExpiryDays)); err != nil {
		return err
	}
	if _, err := stmt.Exec("admin_allowed_networks", s.AdminAllowedNetworks); err != nil {
		return err
	}
	if _, err := stmt.Exec("gateway_allowed_networks", s.GatewayAllowedNetworks); err != nil {
		return err
	}

	return tx.Commit()
}
