package com.smartagriculture.alert.domain;

import com.smartagriculture.farm.domain.Plot;
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

@Entity
@Table(name = "alert_rules")
public class AlertRule extends AbstractAuditableEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "plot_id", nullable = false)
    private Plot plot;

    @Column(nullable = false, length = 128)
    private String name;

    @Column(nullable = false, length = 64)
    private String metric;

    @Enumerated(EnumType.STRING)
    @Column(name = "comparison_operator", nullable = false, length = 16)
    private ComparisonOperator operator;

    @Column(nullable = false, precision = 14, scale = 4)
    private BigDecimal threshold;

    @Column(name = "duration_seconds", nullable = false)
    private int durationSeconds;

    @Column(nullable = false, precision = 14, scale = 4)
    private BigDecimal hysteresis = BigDecimal.ZERO;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 16)
    private AlertLevel level = AlertLevel.WARNING;

    @Column(nullable = false)
    private boolean enabled = true;

    protected AlertRule() {
    }

    public AlertRule(
            Plot plot,
            String name,
            String metric,
            ComparisonOperator operator,
            BigDecimal threshold,
            int durationSeconds) {
        this.plot = plot;
        this.name = name;
        this.metric = metric;
        this.operator = operator;
        this.threshold = threshold;
        this.durationSeconds = durationSeconds;
    }

    public Long getId() {
        return id;
    }
}
