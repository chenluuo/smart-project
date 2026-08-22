package com.smartagriculture.alert.repository;

import com.smartagriculture.alert.domain.AlertRule;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface AlertRuleRepository extends JpaRepository<AlertRule, Long> {

    List<AlertRule> findByPlotIdAndEnabledTrue(Long plotId);
}
