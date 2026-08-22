package com.smartagriculture.device.domain;

import com.smartagriculture.shared.persistence.AbstractAuditableEntity;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import java.time.Instant;

@Entity
@Table(name = "devices")
public class Device extends AbstractAuditableEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "device_code", nullable = false, unique = true, length = 64)
    private String deviceCode;

    @Column(name = "serial_no", nullable = false, unique = true, length = 128)
    private String serialNo;

    @Column(name = "device_type", nullable = false, length = 64)
    private String deviceType;

    @Column(length = 64)
    private String model;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private DeviceStatus status = DeviceStatus.UNACTIVATED;

    @Enumerated(EnumType.STRING)
    @Column(name = "credential_status", nullable = false, length = 32)
    private CredentialStatus credentialStatus = CredentialStatus.PENDING;

    @Column(name = "activated_at")
    private Instant activatedAt;

    @Column(name = "last_seen_at")
    private Instant lastSeenAt;

    protected Device() {
    }

    public Device(String deviceCode, String serialNo, String deviceType) {
        this.deviceCode = deviceCode;
        this.serialNo = serialNo;
        this.deviceType = deviceType;
    }

    public Long getId() {
        return id;
    }

    public String getDeviceCode() {
        return deviceCode;
    }

    public DeviceStatus getStatus() {
        return status;
    }

    public Instant getLastSeenAt() {
        return lastSeenAt;
    }
}
