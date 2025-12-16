CREATE TABLE IF NOT EXISTS Users (
    user_id INT PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    balance DECIMAL(10, 2) DEFAULT 0.00,
    current_points INT DEFAULT 0
);

CREATE TABLE IF NOT EXISTS Transactions (
    transaction_id INT PRIMARY KEY,
    user_id INT NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) CHECK (status IN ('Pending','Paid','Voided','Refunded')) NOT NULL,
    point_change INT DEFAULT 0,
    source_transaction_id INT DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES Users(user_id),
    FOREIGN KEY (source_transaction_id) REFERENCES Transactions(transaction_id)
);