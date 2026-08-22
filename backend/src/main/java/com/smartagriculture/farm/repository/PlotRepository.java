package com.smartagriculture.farm.repository;

import com.smartagriculture.farm.domain.Plot;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface PlotRepository extends JpaRepository<Plot, Long> {

    List<Plot> findByFarmId(Long farmId);
}
