package com.smartagriculture.outbox.repository;

import com.smartagriculture.outbox.domain.OutboxEvent;
import com.smartagriculture.outbox.domain.OutboxStatus;
import java.time.Instant;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface OutboxEventRepository extends JpaRepository<OutboxEvent, Long> {

    List<OutboxEvent> findTop100ByStatusAndAvailableAtLessThanEqualOrderById(
            OutboxStatus status, Instant availableAt);
}
