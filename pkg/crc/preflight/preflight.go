package preflight

import (
	"fmt"

	"github.com/code-ready/crc/pkg/crc/config"
	"github.com/code-ready/crc/pkg/crc/logging"
)

type Flags uint32

// EnableExperimentalFeatures enables the use of experimental features
var EnableExperimentalFeatures bool

const (
	// Indicates a PreflightCheck should only be run as part of "crc setup"
	SetupOnly Flags = 1 << iota
	// Indicates a PreflightCheck should only be run as part of "crc start"
	StartOnly
	NoFix
	CleanUpOnly
)

type CheckFunc func() error
type FixFunc func() error
type CleanUpFunc func() error

type Check struct {
	configKeySuffix    string
	checkDescription   string
	check              CheckFunc
	fixDescription     string
	fix                FixFunc
	flags              Flags
	cleanupDescription string
	cleanup            CleanUpFunc
}

func (check *Check) getSkipConfigName() string {
	if check.configKeySuffix == "" {
		return ""
	}
	return "skip-" + check.configKeySuffix
}

func (check *Check) shouldSkip(config config.Storage) bool {
	if check.configKeySuffix == "" {
		return false
	}
	return config.Get(check.getSkipConfigName()).AsBool()
}

func (check *Check) getWarnConfigName() string {
	if check.configKeySuffix == "" {
		return ""
	}
	return "warn-" + check.configKeySuffix
}

func (check *Check) shouldWarn(config config.Storage) bool {
	if check.configKeySuffix == "" {
		return false
	}
	return config.Get(check.getWarnConfigName()).AsBool()
}

func (check *Check) doCheck(config config.Storage) error {
	if check.checkDescription == "" {
		panic(fmt.Sprintf("Should not happen, empty description for check '%s'", check.configKeySuffix))
	} else {
		logging.Infof("%s", check.checkDescription)
	}
	if check.shouldSkip(config) {
		logging.Warn("Skipping above check ...")
		return nil
	}

	err := check.check()
	if err != nil {
		logging.Debug(err.Error())
	}
	return err
}

func (check *Check) doFix() error {
	if check.fixDescription == "" {
		panic(fmt.Sprintf("Should not happen, empty description for fix '%s'", check.configKeySuffix))
	}
	if check.flags&NoFix == NoFix {
		return fmt.Errorf(check.fixDescription)
	}

	logging.Infof("%s", check.fixDescription)

	return check.fix()
}

func (check *Check) doCleanUp() error {
	if check.cleanupDescription == "" {
		panic(fmt.Sprintf("Should not happen, empty description for cleanup '%s'", check.configKeySuffix))
	}

	logging.Infof("%s", check.cleanupDescription)

	return check.cleanup()
}

func doPreflightChecks(config config.Storage, checks []Check) error {
	for _, check := range checks {
		if check.flags&SetupOnly == SetupOnly || check.flags&CleanUpOnly == CleanUpOnly {
			continue
		}
		err := check.doCheck(config)
		if err != nil {
			if check.shouldWarn(config) {
				logging.Warn(err.Error())
			} else {
				return err
			}
		}
	}
	return nil
}

func doFixPreflightChecks(config config.Storage, checks []Check) error {
	for _, check := range checks {
		if check.flags&StartOnly == StartOnly || check.flags&CleanUpOnly == CleanUpOnly {
			continue
		}
		err := check.doCheck(config)
		if err == nil {
			continue
		}
		err = check.doFix()
		if err != nil {
			if check.shouldWarn(config) {
				logging.Warn(err.Error())
			} else {
				return err
			}
		}
	}
	return nil
}

func doCleanUpPreflightChecks(checks []Check) error {
	// Do the cleanup in reverse order to avoid any dependency during cleanup
	for i := len(checks) - 1; i >= 0; i-- {
		check := checks[i]
		if check.cleanup == nil {
			continue
		}
		err := check.doCleanUp()
		if err != nil {
			return err
		}
	}
	return nil
}

func doRegisterSettings(cfg config.Schema, checks []Check) {
	for _, check := range checks {
		if check.configKeySuffix != "" {
			cfg.AddSetting(check.getSkipConfigName(), false, config.ValidateBool, config.SuccessfullyApplied)
			cfg.AddSetting(check.getWarnConfigName(), false, config.ValidateBool, config.SuccessfullyApplied)
		}
	}
}

// StartPreflightChecks performs the preflight checks before starting the cluster
func StartPreflightChecks(config config.Storage) error {
	return doPreflightChecks(config, getPreflightChecks())
}

// SetupHost performs the prerequisite checks and setups the host to run the cluster
func SetupHost(config config.Storage) error {
	return doFixPreflightChecks(config, getPreflightChecks())
}

func RegisterSettings(config config.Schema) {
	doRegisterSettings(config, getPreflightChecks())
}
