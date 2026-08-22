package com.smartagriculture.audit.repository;

import com.smartagriculture.audit.domain.AuditLog;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface AuditLogRepository extends JpaRepository<AuditLog, Long> {

    List<AuditLog> findByFarmIdOrderByCreatedAtDesc(Long farmId);
}
