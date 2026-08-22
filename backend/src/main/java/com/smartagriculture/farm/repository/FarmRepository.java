package com.smartagriculture.farm.repository;

import com.smartagriculture.farm.domain.Farm;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface FarmRepository extends JpaRepository<Farm, Long> {

    List<Farm> findByOwnerId(Long ownerId);
}
