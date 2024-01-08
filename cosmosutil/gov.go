package cosmosutil

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

func (c *ChainClient) GovProposal(ctx context.Context, proposalID uint64) (*govtypes.Proposal, error) {
	govClient, err := c.getGovClient(ctx)

	if err != nil {
		return nil, err
	}

	res, err := govClient.Proposal(ctx, &govtypes.QueryProposalRequest{
		ProposalId: proposalID,
	})

	if err != nil {
		return nil, err
	}

	return res.GetProposal(), nil
}

func (c *ChainClient) GovProposals(ctx context.Context) ([]*govtypes.Proposal, error) {
	govClient, err := c.getGovClient(ctx)

	if err != nil {
		return nil, err
	}

	var nextToken []byte
	var proposals []*govtypes.Proposal

	for {
		res, err := govClient.Proposals(ctx, &govtypes.QueryProposalsRequest{
			Pagination: &query.PageRequest{
				Key: nextToken,
			},
		})

		if err != nil {
			return nil, err
		}

		proposals = append(proposals, res.Proposals...)

		nextToken = res.Pagination.GetNextKey()
		if nextToken == nil {
			break
		}
	}

	return proposals, nil
}

func (c *ChainClient) GovProposalVotes(ctx context.Context, proposalID uint64) ([]*govtypes.Vote, error) {
	govClient, err := c.getGovClient(ctx)

	if err != nil {
		return nil, err
	}

	var nextToken []byte
	var votes []*govtypes.Vote

	for {
		res, err := govClient.Votes(ctx, &govtypes.QueryVotesRequest{
			ProposalId: proposalID,
			Pagination: &query.PageRequest{
				Key: nextToken,
			},
		})

		if err != nil {
			return nil, err
		}

		votes = append(votes, res.GetVotes()...)

		nextToken = res.Pagination.GetNextKey()
		if nextToken == nil {
			break
		}
	}

	return votes, nil
}

func (c *ChainClient) GovProposalVote(ctx context.Context, proposalID uint64, voter string) (*govtypes.Vote, error) {
	govClient, err := c.getGovClient(ctx)

	if err != nil {
		return nil, err
	}

	res, err := govClient.Vote(ctx, &govtypes.QueryVoteRequest{
		ProposalId: proposalID,
		Voter:      voter,
	})

	if err != nil {
		return nil, err
	}

	return res.GetVote(), nil
}

func (c *ChainClient) GovProposalDeposits(ctx context.Context, proposalID uint64) ([]*govtypes.Deposit, error) {
	govClient, err := c.getGovClient(ctx)

	if err != nil {
		return nil, err
	}

	var nextToken []byte
	var deposits []*govtypes.Deposit

	for {
		res, err := govClient.Deposits(ctx, &govtypes.QueryDepositsRequest{
			ProposalId: proposalID,
			Pagination: &query.PageRequest{
				Key: nextToken,
			},
		})

		if err != nil {
			return nil, err
		}

		deposits = append(deposits, res.GetDeposits()...)

		nextToken = res.Pagination.GetNextKey()
		if nextToken == nil {
			break
		}
	}

	return deposits, nil
}

func (c *ChainClient) GovProposalDeposit(ctx context.Context, proposalID uint64, depositor string) (*govtypes.Deposit, error) {
	govClient, err := c.getGovClient(ctx)

	if err != nil {
		return nil, err
	}

	res, err := govClient.Deposit(ctx, &govtypes.QueryDepositRequest{
		ProposalId: proposalID,
		Depositor:  depositor,
	})

	if err != nil {
		return nil, err
	}

	return res.GetDeposit(), nil
}

func (c *ChainClient) GovTallyResult(ctx context.Context, proposalID uint64) (*govtypes.TallyResult, error) {
	govClient, err := c.getGovClient(ctx)

	if err != nil {
		return nil, err
	}

	res, err := govClient.TallyResult(ctx, &govtypes.QueryTallyResultRequest{
		ProposalId: proposalID,
	})

	if err != nil {
		return nil, err
	}

	return res.GetTally(), nil
}

func (c *ChainClient) GovVoteOnProposal(ctx context.Context, proposalID uint64, voter InteractingWallet, option govtypes.VoteOption) (*sdk.TxResponse, error) {
	msg := govtypes.NewMsgVote(sdk.AccAddress(voter.FormattedAddress()), proposalID, option, "")

	txResp, err := voter.CreateAndBroadcastTx(ctx, true, 0, sdk.Coins{}, 0, msg)

	if err != nil {
		return nil, err
	}

	return txResp, err
}

func (c *ChainClient) GovDepositOnProposal(ctx context.Context, proposalID uint64, depositor InteractingWallet, amount sdk.Coins) (*sdk.TxResponse, error) {
	msg := govtypes.NewMsgDeposit(sdk.AccAddress(depositor.FormattedAddress()), proposalID, amount)

	txResp, err := depositor.CreateAndBroadcastTx(ctx, true, 0, sdk.Coins{}, 0, msg)

	if err != nil {
		return nil, err
	}

	return txResp, err
}

func (c *ChainClient) GovSubmitProposal(ctx context.Context, proposer *InteractingWallet,
	messages []sdk.Msg, initialDeposit sdk.Coins, metadata,
	title, summary string, expedited bool) (*sdk.TxResponse, error) {

	msg, err := govtypes.NewMsgSubmitProposal(
		messages, initialDeposit, proposer.FormattedAddress(), metadata, title, summary, expedited)

	if err != nil {
		return nil, err
	}

	txResp, err := proposer.CreateAndBroadcastTx(ctx, true, 0, sdk.Coins{}, 0, msg)

	if err != nil {
		return nil, err
	}

	return txResp, err
}
