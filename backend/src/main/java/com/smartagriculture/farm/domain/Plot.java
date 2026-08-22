package com.smartagriculture.farm.domain;

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
@Table(name = "plots")
public class Plot extends AbstractAuditableEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "farm_id", nullable = false)
    private Farm farm;

    @Column(nullable = false, length = 128)
    private String name;

    @Column(name = "crop_type", length = 64)
    private String cropType;

    @Column(name = "growth_stage", length = 64)
    private String growthStage;

    @Column(precision = 12, scale = 2)
    private BigDecimal area;

    @Column(length = 255)
    private String location;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private PlotStatus status = PlotStatus.ACTIVE;

    protected Plot() {
    }

    public Plot(Farm farm, String name) {
        this.farm = farm;
        this.name = name;
    }

    public Long getId() {
        return id;
    }

    public Farm getFarm() {
        return farm;
    }

    public String getName() {
        return name;
    }
}
