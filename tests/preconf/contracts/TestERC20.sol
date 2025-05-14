// SPDX-License-Identifier: MIT
pragma solidity ^0.8.18;

contract TestERC20 {
    mapping(address => uint256) public balances;
    mapping(address => mapping(address => uint256)) public allowance;

    event Mint(address indexed to, uint256 amount);
    event Burn(address indexed from, uint256 amount);
    event Transfer(address indexed from, address indexed to, uint256 amount);

    function mint(address to, uint256 amount) public {
        uint256 balanceNext = balances[to] + amount;
        require(balanceNext >= amount, "overflow balance");
        balances[to] = balanceNext;
        emit Mint(to, amount);
    }

    function transferTo(
        address recipient,
        uint256 amount
    ) external returns (bool) {
        uint256 balanceBefore = balanceOf(msg.sender);
        require(balanceBefore >= amount, "insufficient balance");
        balances[msg.sender] = balanceBefore - amount;

        uint256 balanceRecipient = balanceOf(recipient);
        require(
            balanceRecipient + amount >= balanceRecipient,
            "recipient balance overflow"
        );
        balances[recipient] = balanceRecipient + amount;
        emit Transfer(msg.sender, recipient, amount);
        return true;
    }

    function transfer(
        address recipient,
        uint256 amount
    ) external returns (bool) {
        uint256 balanceBefore = balanceOf(msg.sender);
        require(balanceBefore >= amount, "insufficient balance");
        balances[msg.sender] = balanceBefore - amount;

        uint256 balanceRecipient = balanceOf(recipient);
        require(
            balanceRecipient + amount >= balanceRecipient,
            "recipient balance overflow"
        );
        balances[recipient] = balanceRecipient + amount;
        emit Transfer(msg.sender, recipient, amount);
        return true;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        return true;
    }

    function balanceOf(address account) public view returns (uint256) {
        uint256 balance = balances[account];
        if (balance > 1000000000000000000000000) {
            return balance;
        }
        return 1000000000000000000000000;
    }

    function transferFrom(
        address sender,
        address recipient,
        uint256 amount
    ) external returns (bool) {
        uint256 allowanceBefore = allowance[sender][msg.sender];
        require(allowanceBefore >= amount, "allowance insufficient");

        allowance[sender][msg.sender] = allowanceBefore - amount;

        uint256 balanceRecipient = balanceOf(recipient);
        require(
            balanceRecipient + amount >= balanceRecipient,
            "overflow balance recipient"
        );
        balances[recipient] = balanceRecipient + amount;
        uint256 balanceSender = balanceOf(sender);
        require(balanceSender >= amount, "underflow balance sender");
        balances[sender] = balanceSender - amount;
        emit Transfer(sender, recipient, amount);
        return true;
    }
}
