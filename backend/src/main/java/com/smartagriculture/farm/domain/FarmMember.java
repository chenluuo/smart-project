package com.smartagriculture.farm.domain;

import com.smartagriculture.identity.domain.UserAccount;
import com.smartagriculture.shared.persistence.AbstractAuditableEntity;
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
import jakarta.persistence.UniqueConstraint;
import jakarta.persistence.Column;

@Entity
@Table(
        name = "farm_users",
        uniqueConstraints = @UniqueConstraint(name = "uk_farm_user", columnNames = {"farm_id", "user_id"}))
public class FarmMember extends AbstractAuditableEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "farm_id", nullable = false)
    private Farm farm;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "user_id", nullable = false)
    private UserAccount user;

    @Enumerated(EnumType.STRING)
    @Column(name = "farm_role", nullable = false, length = 32)
    private FarmRole farmRole;

    protected FarmMember() {
    }

    public FarmMember(Farm farm, UserAccount user, FarmRole farmRole) {
        this.farm = farm;
        this.user = user;
        this.farmRole = farmRole;
    }

    public Long getId() {
        return id;
    }
}
