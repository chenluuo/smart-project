package com.smartagriculture.control.domain;

public enum CommandStatus {
    PENDING,
    REJECTED,
    SENT,
    ACKNOWLEDGED,
    SUCCEEDED,
    FAILED,
    TIMEOUT,
    EXPIRED
}
