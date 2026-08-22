package com.smartagriculture.alert.domain;

import com.smartagriculture.device.domain.Device;
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
import java.math.BigDecimal;
import java.time.Instant;

@Entity
@Table(name = "alerts")
public class Alert extends AbstractAuditableEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "rule_id", nullable = false)
    private AlertRule rule;

    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "device_id")
    private Device device;

    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "acknowledged_by")
    private UserAccount acknowledgedBy;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 16)
    private AlertLevel level;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private AlertStatus status = AlertStatus.ACTIVE;

    @Column(name = "trigger_value", nullable = false, precision = 14, scale = 4)
    private BigDecimal triggerValue;

    @Column(name = "triggered_at", nullable = false)
    private Instant triggeredAt;

    @Column(name = "acknowledged_at")
    private Instant acknowledgedAt;

    @Column(name = "resolved_at")
    private Instant resolvedAt;

    protected Alert() {
    }

    public Alert(AlertRule rule, Device device, AlertLevel level, BigDecimal triggerValue, Instant triggeredAt) {
        this.rule = rule;
        this.device = device;
        this.level = level;
        this.triggerValue = triggerValue;
        this.triggeredAt = triggeredAt;
    }

    public Long getId() {
        return id;
    }

    public AlertStatus getStatus() {
        return status;
    }
}
