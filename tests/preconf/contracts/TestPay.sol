// SPDX-License-Identifier: MIT
pragma solidity ^0.8.18;

import {TestERC20} from "./TestERC20.sol";

contract TestPay {
    TestERC20 erc20;

    function setTestERC20(address _erc20) external {
        erc20 = TestERC20(_erc20);
    }

    function transferTo(address sender, address recipient, uint256 amount) external returns (bool) {
        erc20.transferFrom(sender, recipient, amount);
        return true;
    }
}
