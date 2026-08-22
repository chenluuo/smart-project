package com.smartagriculture.device.domain;

import com.smartagriculture.farm.domain.Plot;
import com.smartagriculture.identity.domain.UserAccount;
import jakarta.persistence.Entity;
import jakarta.persistence.FetchType;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.JoinColumn;
import jakarta.persistence.ManyToOne;
import jakarta.persistence.Table;
import java.time.Instant;
import org.hibernate.annotations.CreationTimestamp;

@Entity
@Table(name = "device_bindings")
public class DeviceBinding {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "device_id", nullable = false)
    private Device device;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "plot_id", nullable = false)
    private Plot plot;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "bound_by", nullable = false)
    private UserAccount boundBy;

    @CreationTimestamp
    @jakarta.persistence.Column(name = "bound_at", nullable = false, updatable = false)
    private Instant boundAt;

    @jakarta.persistence.Column(name = "unbound_at")
    private Instant unboundAt;

    protected DeviceBinding() {
    }

    public DeviceBinding(Device device, Plot plot, UserAccount boundBy) {
        this.device = device;
        this.plot = plot;
        this.boundBy = boundBy;
    }

    public Long getId() {
        return id;
    }

    public boolean isActive() {
        return unboundAt == null;
    }
}
