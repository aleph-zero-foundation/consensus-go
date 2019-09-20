package add

//import (
//	"github.com/rs/zerolog"
//	"gitlab.com/alephledger/consensus-go/pkg/gomel"
//	"gitlab.com/alephledger/consensus-go/pkg/logging"
//)
//
//// LogAddUnitError including the location of where it happens.
//// It returns the logged error, unless it's DuplicateUnit.
//func LogAddUnitError(pu gomel.Preunit, err error, fallback Fallback, location string, log zerolog.Logger) error {
//	if err == nil {
//		return nil
//	}
//	switch e := err.(type) {
//	case *gomel.DuplicateUnit:
//		log.Info().Uint16(logging.Creator, e.Unit.Creator()).Int(logging.Height, e.Unit.Height()).Msg(logging.DuplicatedUnit)
//		return nil
//	case *gomel.UnknownParents:
//		log.Info().Uint16(logging.Creator, pu.Creator()).Int(logging.Size, e.Amount).Msg(logging.UnknownParents)
//		fallback.Run(pu)
//		return err
//	default:
//		log.Error().Str("where", location).Msg(err.Error())
//		return err
//	}
//}
//
