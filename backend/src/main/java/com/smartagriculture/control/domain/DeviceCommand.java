package com.smartagriculture.control.domain;

import com.smartagriculture.device.domain.Device;
import com.smartagriculture.farm.domain.Plot;
import com.smartagriculture.identity.domain.UserAccount;
import com.smartagriculture.shared.persistence.AbstractAuditableEntity;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.FetchType;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.JoinColumn;
import jakarta.persistence.ManyToOne;
import jakarta.persistence.Table;
import java.time.Instant;

@Entity
@Table(name = "device_commands")
public class DeviceCommand extends AbstractAuditableEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "command_id", nullable = false, unique = true, length = 64)
    private String commandId;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "device_id", nullable = false)
    private Device device;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "plot_id", nullable = false)
    private Plot plot;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "issued_by", nullable = false)
    private UserAccount issuedBy;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private CommandAction action;

    @Column(name = "parameters_json", nullable = false, columnDefinition = "json")
    private String parametersJson = "{}";

    @Column(name = "idempotency_key", nullable = false, unique = true, length = 64)
    private String idempotencyKey;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private CommandStatus status = CommandStatus.PENDING;

    @Column(name = "error_code", length = 64)
    private String errorCode;

    @Column(name = "error_message", length = 500)
    private String errorMessage;

    @Column(name = "issued_at", nullable = false)
    private Instant issuedAt;

    @Column(name = "expires_at", nullable = false)
    private Instant expiresAt;

    @Column(name = "executed_at")
    private Instant executedAt;

    protected DeviceCommand() {
    }

    public DeviceCommand(
            String commandId,
            Device device,
            Plot plot,
            UserAccount issuedBy,
            CommandAction action,
            String idempotencyKey,
            Instant issuedAt,
            Instant expiresAt) {
        this.commandId = commandId;
        this.device = device;
        this.plot = plot;
        this.issuedBy = issuedBy;
        this.action = action;
        this.idempotencyKey = idempotencyKey;
        this.issuedAt = issuedAt;
        this.expiresAt = expiresAt;
    }

    public Long getId() {
        return id;
    }

    public String getCommandId() {
        return commandId;
    }

    public CommandStatus getStatus() {
        return status;
    }
}
