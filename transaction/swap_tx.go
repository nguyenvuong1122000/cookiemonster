package transaction

import (
	"errors"
	"fmt"
	"os"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/notional-labs/cookiemonster/osmosis"
	"github.com/notional-labs/cookiemonster/query"
	"github.com/osmosis-labs/osmosis/x/gamm/types"
	"gopkg.in/yaml.v3"
)

type SwapOption struct {
	SwapRoutePoolIds []int
	SwapRouteDenoms  []string
	TokenInAmount    sdk.Int
	TokenInDenom     string
	TokenOutMinAmt   sdk.Int
}

func swapAmountInRoutes(swapOpt SwapOption) ([]types.SwapAmountInRoute, error) {
	swapRoutePoolIds := swapOpt.SwapRoutePoolIds
	swapRouteDenoms := swapOpt.SwapRouteDenoms

	if len(swapRoutePoolIds) != len(swapRouteDenoms) {
		return nil, errors.New("swap route pool ids and denoms mismatch")
	}
	routes := []types.SwapAmountInRoute{}
	for index, poolID := range swapRoutePoolIds {
		routes = append(routes, types.SwapAmountInRoute{
			PoolId:        uint64(poolID),
			TokenOutDenom: swapRouteDenoms[index],
		})
	}
	return routes, nil
}

func NewMsgSwapExactAmountIn(fromAddr sdk.AccAddress, swapOpt SwapOption) (sdk.Msg, error) {
	routes, err := swapAmountInRoutes(swapOpt)
	if err != nil {
		return nil, err
	}

	tokenIn := sdk.Coin{Denom: swapOpt.TokenInDenom, Amount: swapOpt.TokenInAmount}

	tokenOutMinAmt := swapOpt.TokenOutMinAmt

	msg := &types.MsgSwapExactAmountIn{
		Sender:            fromAddr.String(),
		Routes:            routes,
		TokenIn:           tokenIn,
		TokenOutMinAmount: tokenOutMinAmt,
	}
	return msg, nil
}

func Swap(keyName string, swapOpt SwapOption, gas uint64) (string, error) {
	// build tx context
	clientCtx := osmosis.DefaultClientCtx
	clientCtx, err := SetKeyNameToContext(clientCtx, keyName)
	if err != nil {
		return "", err
	}

	txf := NewTxFactoryFromClientCtx(clientCtx).WithGas(gas)

	// build msg for tx
	fromAddr := clientCtx.GetFromAddress()
	msg, err := NewMsgSwapExactAmountIn(fromAddr, swapOpt)
	if err != nil {
		return "", err
	}

	code, txHash, err := BroadcastTx(clientCtx, txf, msg)
	if err != nil {
		return txHash, err
	}
	if code != 0 {
		return txHash, fmt.Errorf("tx failed with code %d", code)
	}
	broadcastedTx, err := query.QueryTx(txHash)
	if err != nil {
		return txHash, err
	}
	if broadcastedTx.Code == 11 {
		return txHash, fmt.Errorf("insufficient fee")

	}
	if broadcastedTx.Code != 0 {
		return txHash, fmt.Errorf("tx failed with code %d", code)
	}
	return txHash, nil
}

type SwapTx struct {
	KeyName string
	SwapOpt SwapOption
}

func (swapTx SwapTx) Execute() (string, error) {

	keyName := swapTx.KeyName
	swapOpt := swapTx.SwapOpt
	gas := 200000
	var err error
	var txHash string

	// if tx failed because of insufficient fee , retry
	for i := 0; i < 4; i++ {
		txHash, err = Swap(keyName, swapOpt, uint64(gas))
		if err == nil {
			return txHash, nil
		}
		if err.Error() != "insufficient fee" {
			return txHash, err
		}
		gas += 300000
	}
	return txHash, err
}

func (swapTx SwapTx) Report() {

	swapOpt := swapTx.SwapOpt
	keyName := swapTx.KeyName

	f, _ := os.OpenFile("report", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)

	f.WriteString("\nSwap Transaction\n")
	f.WriteString("\nKeyname: " + keyName + "\n")
	f.WriteString("\nSwap Option\n\n")

	txData, _ := yaml.Marshal(swapOpt)
	_, _ = f.Write(txData)
	f.WriteString(transactionSeperator)

	f.Close()
}

func (swapTx SwapTx) Prompt() {
	swapOpt := swapTx.SwapOpt
	keyName := swapTx.KeyName

	fmt.Print("\nSwap Transaction\n")
	fmt.Print("\nKeyname: " + keyName + "\n")
	fmt.Print("\nSwap Option\n\n")
	fmt.Printf("%+v\n", swapOpt)
	fmt.Print(transactionSeperator)
}
