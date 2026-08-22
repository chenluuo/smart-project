package com.smartagriculture.alert.repository;

import com.smartagriculture.alert.domain.Alert;
import com.smartagriculture.alert.domain.AlertStatus;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface AlertRepository extends JpaRepository<Alert, Long> {

    List<Alert> findByRulePlotFarmIdAndStatusIn(Long farmId, List<AlertStatus> statuses);
}
